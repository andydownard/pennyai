package main

// Test database.
//
// Every test runs on a real PostgreSQL server. TEST_DATABASE_URL names an admin
// database on a server where the test user may CREATE DATABASE; it defaults to
// the local Homebrew server:
//
//	TEST_DATABASE_URL=postgres://localhost:5432/postgres?sslmode=disable go test -count=1 ./...
//
// The production schema (initDB) is built once into a template database; each
// test then gets its own copy, dropped on cleanup.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
)

const pgTestCompatDriver = "pgx-testcompat"

var (
	pgTemplateOnce sync.Once
	pgTemplateName string
	pgTemplateErr  error
	pgTestDBSeq    atomic.Int64
)

func init() {
	sql.Register(pgTestCompatDriver, &pgCompatDriver{inner: stdlib.GetDefaultDriver()})
}

const defaultTestDatabaseURL = "postgres://localhost:5432/postgres?sslmode=disable"

// postgresTestURL returns the admin URL the suite runs against.
func postgresTestURL() string {
	if u := os.Getenv("TEST_DATABASE_URL"); u != "" {
		return u
	}
	return defaultTestDatabaseURL
}

// pgURLForDatabase swaps the database name in a postgres:// URL.
func pgURLForDatabase(adminURL, name string) (string, error) {
	u, err := url.Parse(adminURL)
	if err != nil {
		return "", err
	}
	u.Path = "/" + name
	// Production runs with a UTC session zone. The app writes UTC wall-clock
	// strings ("2006-01-02 15:04:05") into TIMESTAMPTZ columns, which only
	// round-trips when the session zone is UTC, so the tests pin it too.
	q := u.Query()
	q.Set("timezone", "UTC")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// pgBuildTemplate creates the template database and runs the production
// schema into it. Runs once per test process.
func pgBuildTemplate(adminURL string) (string, error) {
	admin, err := sql.Open("pgx", adminURL)
	if err != nil {
		return "", err
	}
	defer admin.Close()
	if err := admin.Ping(); err != nil {
		return "", fmt.Errorf("cannot reach PostgreSQL at %s (%v).\nThe Go tests need a PostgreSQL server where the user may CREATE DATABASE.\nStart one (brew services start postgresql@16) or set TEST_DATABASE_URL, e.g.\nTEST_DATABASE_URL=postgres://user:pass@host:5432/postgres?sslmode=disable go test -count=1 ./...", adminURL, err)
	}

	name := fmt.Sprintf("pennyai_test_tmpl_%d", os.Getpid())
	if _, err := admin.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)"); err != nil {
		return "", fmt.Errorf("dropping stale template: %w", err)
	}
	if _, err := admin.Exec("CREATE DATABASE " + name); err != nil {
		return "", fmt.Errorf("creating template: %w", err)
	}

	tmplURL, err := pgURLForDatabase(adminURL, name)
	if err != nil {
		return "", err
	}
	tmpl, err := sql.Open("pgx", tmplURL)
	if err != nil {
		return "", err
	}
	// initDB must run on the plain driver: it is production DDL, and the
	// template can only be copied once every connection to it is closed.
	if err := initDB(tmpl); err != nil {
		tmpl.Close()
		return "", fmt.Errorf("initDB on template: %w", err)
	}
	// Fixtures insert rows with explicit small ids, which
	// does not advance a PostgreSQL sequence. Start every sequence well above
	// them so generated ids never collide with fixture ids.
	seqs, err := tmpl.Query("SELECT sequencename FROM pg_sequences WHERE schemaname = current_schema()")
	if err != nil {
		tmpl.Close()
		return "", err
	}
	var names []string
	for seqs.Next() {
		var n string
		if seqs.Scan(&n) == nil {
			names = append(names, n)
		}
	}
	seqs.Close()
	for _, n := range names {
		if _, err := tmpl.Exec(fmt.Sprintf(`ALTER SEQUENCE %q RESTART WITH 100000`, n)); err != nil {
			tmpl.Close()
			return "", fmt.Errorf("bumping sequence %s: %w", n, err)
		}
	}
	if err := tmpl.Close(); err != nil {
		return "", err
	}
	return name, nil
}

// openPostgresTestDB gives the test its own database cloned from the template.
// The raw handle converts `?` placeholders so test fixtures written in the
// legacy dialect keep working; production code must not rely on that — it goes through PGDB.
func openPostgresTestDB(t *testing.T, adminURL string) (*sql.DB, *PGDB) {
	t.Helper()
	pgTemplateOnce.Do(func() {
		pgTemplateName, pgTemplateErr = pgBuildTemplate(adminURL)
	})
	if pgTemplateErr != nil {
		t.Fatalf("Postgres test tier: %v", pgTemplateErr)
	}

	admin, err := sql.Open("pgx", adminURL)
	if err != nil {
		t.Fatalf("Postgres test tier: %v", err)
	}
	name := fmt.Sprintf("pennyai_test_%d_%d", os.Getpid(), pgTestDBSeq.Add(1))
	if _, err := admin.Exec("CREATE DATABASE " + name + " TEMPLATE " + pgTemplateName); err != nil {
		admin.Close()
		t.Fatalf("Postgres test tier: cloning template: %v", err)
	}

	dbURL, err := pgURLForDatabase(adminURL, name)
	if err != nil {
		admin.Close()
		t.Fatalf("Postgres test tier: %v", err)
	}
	db, err := sql.Open(pgTestCompatDriver, dbURL)
	if err != nil {
		admin.Close()
		t.Fatalf("Postgres test tier: %v", err)
	}
	db.SetMaxOpenConns(10)

	t.Cleanup(func() {
		db.Close()
		admin.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)")
		admin.Close()
	})
	return db, WrapPG(db)
}

// TestMain drops the template database after the run.
func TestMain(m *testing.M) {
	code := m.Run()
	if pgTemplateName != "" {
		if admin, err := sql.Open("pgx", postgresTestURL()); err == nil {
			admin.Exec("DROP DATABASE IF EXISTS " + pgTemplateName + " WITH (FORCE)")
			admin.Close()
		}
	}
	os.Exit(code)
}

// -----------------------------------------------------------------------------
// pgCompatDriver wraps pgx so a raw *sql.DB accepts legacy-dialect (`?`) SQL. It exists
// only for test fixtures.
// -----------------------------------------------------------------------------

type pgCompatDriver struct{ inner driver.Driver }

func (d *pgCompatDriver) Open(name string) (driver.Conn, error) {
	c, err := d.inner.Open(name)
	if err != nil {
		return nil, err
	}
	return &pgCompatConn{Conn: c}, nil
}

type pgCompatConn struct{ driver.Conn }

func (c *pgCompatConn) Prepare(query string) (driver.Stmt, error) {
	return c.Conn.Prepare(convertSQL(query))
}

func (c *pgCompatConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	return c.Conn.(driver.ConnPrepareContext).PrepareContext(ctx, convertSQL(query))
}

func (c *pgCompatConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return c.Conn.(driver.ExecerContext).ExecContext(ctx, convertSQL(query), args)
}

func (c *pgCompatConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return c.Conn.(driver.QueryerContext).QueryContext(ctx, convertSQL(query), args)
}

func (c *pgCompatConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	return c.Conn.(driver.ConnBeginTx).BeginTx(ctx, opts)
}

func (c *pgCompatConn) Ping(ctx context.Context) error {
	return c.Conn.(driver.Pinger).Ping(ctx)
}

func (c *pgCompatConn) ResetSession(ctx context.Context) error {
	if r, ok := c.Conn.(driver.SessionResetter); ok {
		return r.ResetSession(ctx)
	}
	return nil
}

func (c *pgCompatConn) IsValid() bool {
	if v, ok := c.Conn.(driver.Validator); ok {
		return v.IsValid()
	}
	return true
}

func (c *pgCompatConn) CheckNamedValue(nv *driver.NamedValue) error {
	if ch, ok := c.Conn.(driver.NamedValueChecker); ok {
		return ch.CheckNamedValue(nv)
	}
	return driver.ErrSkip
}
