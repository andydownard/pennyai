// Lightweight in-memory Supabase stub for testing Netlify functions.
//
// Usage: each test file declares
//   vi.mock('@supabase/supabase-js', () => import('./_supabase-mock'))
// then imports { __mockState, resetMockState } from this file to seed
// rows or assert side-effects.
//
// Supports the chain shapes used by feedback.mts and campaign-lead.mts:
//   from(t).insert(row).select('id').single()
//   from(t).select('id', {count:'exact', head:true}).eq(...).gte(...)
//   from(t).select('cols').eq(col,v).order(col,opts).limit(n)
//   from(t).select('id').eq(col,v).maybeSingle()

interface MockTable {
  rows: Record<string, unknown>[]
  uniqueColumns: string[]
  nextId: number
}

export interface MockSupabaseState {
  tables: Record<string, MockTable>
  // Force the next operation of a given mode to return an error.
  forceError: { op: 'insert' | 'select' | 'count'; error: { message: string; code?: string } } | null
  // Captured calls for assertions
  insertCalls: Array<{ table: string; row: Record<string, unknown> }>
}

function freshState(): MockSupabaseState {
  return {
    tables: {
      feedback_requests: { rows: [], uniqueColumns: [], nextId: 1 },
      campaign_leads: { rows: [], uniqueColumns: ['idempotency_key'], nextId: 1 },
    },
    forceError: null,
    insertCalls: [],
  }
}

export const __mockState: MockSupabaseState = freshState()

export function resetMockState(): void {
  const fresh = freshState()
  __mockState.tables = fresh.tables
  __mockState.forceError = fresh.forceError
  __mockState.insertCalls = fresh.insertCalls
}

class QueryBuilder {
  private filters: Array<{ op: 'eq' | 'gte' | 'lte'; col: string; val: unknown }> = []
  private wantCount = false
  private headOnly = false
  private orderCol: string | null = null
  private orderAsc = true
  private limitN: number | null = null
  private singleMode: 'single' | 'maybeSingle' | null = null
  private insertedRows: Array<Record<string, unknown> & { __conflict__?: string }> = []
  private mode: 'select' | 'insert' = 'select'
  private selectedColumns: string = '*'

  constructor(private tableName: string) {}

  select(cols: string, opts?: { count?: 'exact'; head?: boolean }): this {
    this.selectedColumns = cols
    if (opts?.count) this.wantCount = true
    if (opts?.head) this.headOnly = true
    return this
  }

  insert(row: Record<string, unknown> | Record<string, unknown>[]): this {
    this.mode = 'insert'
    const rows = Array.isArray(row) ? row : [row]
    const table = __mockState.tables[this.tableName]
    if (!table) throw new Error(`Unknown table ${this.tableName}`)
    for (const r of rows) {
      __mockState.insertCalls.push({ table: this.tableName, row: { ...r } })
      let conflictCol: string | undefined
      for (const uc of table.uniqueColumns) {
        if (r[uc] !== undefined && table.rows.some(existing => existing[uc] === r[uc])) {
          conflictCol = uc
          break
        }
      }
      if (conflictCol) {
        this.insertedRows.push({ ...r, __conflict__: conflictCol })
        continue
      }
      const stored = {
        id: table.nextId++,
        created_at: new Date().toISOString(),
        ...r,
      }
      table.rows.push(stored)
      this.insertedRows.push(stored)
    }
    return this
  }

  eq(col: string, val: unknown): this {
    this.filters.push({ op: 'eq', col, val })
    return this
  }

  gte(col: string, val: unknown): this {
    this.filters.push({ op: 'gte', col, val })
    return this
  }

  lte(col: string, val: unknown): this {
    this.filters.push({ op: 'lte', col, val })
    return this
  }

  order(col: string, opts?: { ascending?: boolean }): this {
    this.orderCol = col
    this.orderAsc = opts?.ascending ?? true
    return this
  }

  limit(n: number): this {
    this.limitN = n
    return this
  }

  single(): Promise<{ data: unknown; error: { message: string; code?: string } | null }> {
    this.singleMode = 'single'
    return this.execute()
  }

  maybeSingle(): Promise<{ data: unknown; error: { message: string; code?: string } | null }> {
    this.singleMode = 'maybeSingle'
    return this.execute()
  }

  // Allow `await` on the chain without explicit terminal call
  then<TResult1 = unknown, TResult2 = never>(
    onfulfilled?: (v: { data: unknown; count?: number; error: unknown }) => TResult1 | PromiseLike<TResult1>,
    onrejected?: (reason: unknown) => TResult2 | PromiseLike<TResult2>
  ): Promise<TResult1 | TResult2> {
    return this.execute().then(onfulfilled as never, onrejected as never)
  }

  private async execute(): Promise<{
    data: unknown
    count?: number
    error: { message: string; code?: string } | null
  }> {
    if (__mockState.forceError && __mockState.forceError.op === this.mode) {
      const e = __mockState.forceError.error
      __mockState.forceError = null
      return { data: null, error: e }
    }

    if (this.mode === 'insert') {
      const conflict = this.insertedRows.find(r => r.__conflict__)
      if (conflict) {
        return {
          data: null,
          error: { message: 'duplicate key value violates unique constraint', code: '23505' },
        }
      }
      const inserted = this.insertedRows[0]
      const row = inserted ? this.project(inserted) : null
      return { data: this.singleMode ? row : [row], error: null }
    }

    const table = __mockState.tables[this.tableName]
    if (!table) return { data: null, error: { message: `Unknown table ${this.tableName}` } }

    const matched = table.rows.filter(r =>
      this.filters.every(f => {
        if (f.op === 'eq') return r[f.col] === f.val
        if (f.op === 'gte') return String(r[f.col]) >= String(f.val)
        if (f.op === 'lte') return String(r[f.col]) <= String(f.val)
        return true
      })
    )

    if (this.wantCount) {
      if (__mockState.forceError && __mockState.forceError.op === 'count') {
        const e = __mockState.forceError.error
        __mockState.forceError = null
        return { data: null, count: undefined, error: e }
      }
      return {
        data: this.headOnly ? null : matched.map(r => this.project(r)),
        count: matched.length,
        error: null,
      }
    }

    let result = matched
    if (this.orderCol) {
      const col = this.orderCol
      result = [...result].sort((a, b) => {
        const av = String(a[col] ?? '')
        const bv = String(b[col] ?? '')
        return this.orderAsc ? av.localeCompare(bv) : bv.localeCompare(av)
      })
    }
    if (this.limitN !== null) result = result.slice(0, this.limitN)
    const projected = result.map(r => this.project(r))

    if (this.singleMode === 'single') {
      if (projected.length === 0) {
        return { data: null, error: { message: 'No rows', code: 'PGRST116' } }
      }
      return { data: projected[0], error: null }
    }
    if (this.singleMode === 'maybeSingle') {
      return { data: projected[0] ?? null, error: null }
    }
    return { data: projected, error: null }
  }

  private project(row: Record<string, unknown>): Record<string, unknown> {
    if (this.selectedColumns === '*' || this.selectedColumns.trim() === '') {
      return { ...row }
    }
    const cols = this.selectedColumns.split(',').map(c => c.trim())
    const out: Record<string, unknown> = {}
    for (const c of cols) {
      if (c in row) out[c] = row[c]
    }
    if (cols.includes('id') && 'id' in row) out.id = row.id
    return out
  }
}

export function createClient(_url: string, _key: string) {
  return {
    from: (table: string) => new QueryBuilder(table),
  }
}
