-- Campaign leads table — captures unauthenticated brand interest from
-- the marketing site (groflo.io). Real account creation, deposits, and
-- campaign launch happen in the brand portal (brands.groflo.io); this
-- table is the hand-off point.

create table if not exists campaign_leads (
  id bigint generated always as identity primary key,
  -- Client-supplied idempotency key (uuid v4). Stripe-style: same key in,
  -- same row out. Prevents double-submit / retry from creating duplicates.
  idempotency_key text not null,
  full_name text not null,
  company_name text not null,
  email text not null,
  website text not null default '',
  ad_type text not null,
  categories text not null,           -- comma-separated; portal re-parses
  geography text not null,
  budget_type text not null check (budget_type in ('daily', 'total')),
  budget_amount integer not null check (budget_amount > 0),
  start_date date not null,
  end_date date not null,
  creative_name text not null,
  creative_description text not null,
  status text not null default 'new' check (status in ('new', 'contacted', 'converted', 'dismissed')),
  source text not null default 'campaign_signup',
  ip_hash text not null default '',   -- sha256(client_ip + salt) for rate limiting
  user_agent text not null default '',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),

  constraint campaign_leads_idempotency_key_unique unique (idempotency_key)
);

-- Lookup by email (operator triage)
create index if not exists idx_campaign_leads_email on campaign_leads (lower(email));
create index if not exists idx_campaign_leads_status_created on campaign_leads (status, created_at desc);
-- Per-IP rate limiting: count recent submissions from same IP
create index if not exists idx_campaign_leads_ip_created on campaign_leads (ip_hash, created_at desc);

-- Auto-update updated_at on changes (reuse trigger function from 001)
create trigger trg_campaign_leads_updated_at
  before update on campaign_leads
  for each row execute function update_feedback_updated_at();

-- RLS: no direct client access. Netlify Functions use the service_role key.
alter table campaign_leads enable row level security;
