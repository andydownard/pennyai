/**
 * In-memory mock of the `pg` Pool used by server.ts.
 *
 * Installed via vi.mock('pg', () => import('./_pg-mock')). Implements just
 * enough of the Pool contract that server.ts routes can exercise:
 *   - CREATE TABLE / CREATE INDEX (no-op; schema is implicit in the mock)
 *   - SELECT id FROM campaign_leads WHERE idempotency_key = $1
 *   - SELECT id FROM marketing_feedback_requests WHERE idempotency_key = $1
 *   - SELECT ... FROM marketing_feedback_requests WHERE phone = $1 ORDER BY created_at DESC
 *   - INSERT INTO campaign_leads (...) ON CONFLICT DO NOTHING RETURNING id
 *   - INSERT INTO marketing_feedback_requests (...) ON CONFLICT ... RETURNING id
 *
 * The mock maintains per-table row arrays and honors unique-key constraints
 * on `idempotency_key` so idempotency paths in the handlers are testable.
 */

interface QueryResult<T = unknown> {
  rows: T[]
  rowCount: number
}

interface MockState {
  campaignLeads: Record<string, unknown>[]
  feedbackRequests: Record<string, unknown>[]
  nextId: number
  forceErrorOnNextQuery: boolean
  queries: string[]
}

export const __mockState: MockState = {
  campaignLeads: [],
  feedbackRequests: [],
  nextId: 1,
  forceErrorOnNextQuery: false,
  queries: [],
}

export function resetMockState(): void {
  __mockState.campaignLeads = []
  __mockState.feedbackRequests = []
  __mockState.nextId = 1
  __mockState.forceErrorOnNextQuery = false
  __mockState.queries = []
}

function normalizeSql(sql: string): string {
  return sql.replace(/\s+/g, ' ').trim().toLowerCase()
}

export class Pool {
  constructor(_config?: unknown) {
    // no-op
  }

  on(_event: string, _handler: unknown): this {
    return this
  }

  async query<T = unknown>(sql: string, params: unknown[] = []): Promise<QueryResult<T>> {
    if (__mockState.forceErrorOnNextQuery) {
      __mockState.forceErrorOnNextQuery = false
      throw new Error('mock pg error')
    }

    __mockState.queries.push(sql)
    const q = normalizeSql(sql)

    // DDL — no-op (schema is implicit in the mock)
    if (q.startsWith('create table') || q.startsWith('create index') || q.startsWith('create unique index')) {
      return { rows: [], rowCount: 0 }
    }

    // SELECT id FROM campaign_leads WHERE idempotency_key = $1
    if (q.includes('from campaign_leads') && q.includes('where idempotency_key')) {
      const key = params[0] as string
      const row = __mockState.campaignLeads.find((r) => r.idempotency_key === key)
      return { rows: (row ? [{ id: row.id }] : []) as T[], rowCount: row ? 1 : 0 }
    }

    // SELECT id FROM marketing_feedback_requests WHERE idempotency_key = $1
    if (q.includes('select id from marketing_feedback_requests') && q.includes('where idempotency_key')) {
      const key = params[0] as string
      const row = __mockState.feedbackRequests.find((r) => r.idempotency_key === key)
      return { rows: (row ? [{ id: row.id }] : []) as T[], rowCount: row ? 1 : 0 }
    }

    // SELECT ... FROM marketing_feedback_requests WHERE phone = $1 ORDER BY created_at DESC
    if (q.includes('from marketing_feedback_requests') && q.includes('where phone')) {
      const phone = params[0] as string
      const rows = __mockState.feedbackRequests
        .filter((r) => r.phone === phone)
        .slice()
        .sort((a, b) => String(b.created_at).localeCompare(String(a.created_at)))
        .map((r) => ({
          id: r.id,
          body: r.body,
          urgency: r.urgency,
          status: r.status,
          admin_notes: r.admin_notes,
          created_at: r.created_at,
        }))
      return { rows: rows as T[], rowCount: rows.length }
    }

    // INSERT INTO campaign_leads
    if (q.startsWith('insert into campaign_leads')) {
      const [
        idempotency_key,
        full_name,
        company_name,
        email,
        website,
        ad_type,
        categories,
        geography,
        budget_type,
        budget_amount,
        start_date,
        end_date,
        creative_name,
        creative_description,
        ip_hash,
        user_agent,
      ] = params as string[]
      // ON CONFLICT DO NOTHING — if key exists, return no rows
      if (__mockState.campaignLeads.some((r) => r.idempotency_key === idempotency_key)) {
        return { rows: [], rowCount: 0 }
      }
      const id = __mockState.nextId++
      __mockState.campaignLeads.push({
        id,
        idempotency_key,
        full_name,
        company_name,
        email,
        website,
        ad_type,
        categories,
        geography,
        budget_type,
        budget_amount,
        start_date,
        end_date,
        creative_name,
        creative_description,
        ip_hash,
        user_agent,
        created_at: new Date().toISOString(),
      })
      return { rows: [{ id }] as T[], rowCount: 1 }
    }

    // INSERT INTO marketing_feedback_requests
    if (q.startsWith('insert into marketing_feedback_requests')) {
      const [phone, body, urgency, idempotency_key] = params as string[]
      if (idempotency_key && __mockState.feedbackRequests.some((r) => r.idempotency_key === idempotency_key)) {
        return { rows: [], rowCount: 0 }
      }
      const id = __mockState.nextId++
      __mockState.feedbackRequests.push({
        id,
        phone,
        body,
        urgency,
        status: 'new',
        admin_notes: '',
        idempotency_key: idempotency_key || '',
        created_at: new Date().toISOString(),
      })
      return { rows: [{ id }] as T[], rowCount: 1 }
    }

    throw new Error(`mock pg: unhandled query: ${sql}`)
  }

  async connect(): Promise<{ query: Pool['query']; release: () => void }> {
    return { query: this.query.bind(this), release: () => {} }
  }

  async end(): Promise<void> {
    // no-op
  }
}
