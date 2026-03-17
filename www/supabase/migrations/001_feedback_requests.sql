-- Feedback requests table (matches Rally/PennyAI pattern)
create table if not exists feedback_requests (
  id bigint generated always as identity primary key,
  phone text not null,
  body text not null,
  urgency text not null default 'normal' check (urgency in ('normal', 'critical')),
  status text not null default 'new' check (status in ('new', 'reviewed', 'roadmapped', 'done', 'dismissed')),
  admin_notes text not null default '',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

-- Index for lookups by phone
create index if not exists idx_feedback_phone on feedback_requests (phone);

-- Index for admin triage (newest first, unresolved first)
create index if not exists idx_feedback_status_created on feedback_requests (status, created_at desc);

-- Auto-update updated_at on changes
create or replace function update_feedback_updated_at()
returns trigger as $$
begin
  new.updated_at = now();
  return new;
end;
$$ language plpgsql;

create trigger trg_feedback_updated_at
  before update on feedback_requests
  for each row execute function update_feedback_updated_at();

-- RLS: disable direct client access (all access goes through Netlify Functions with service key)
alter table feedback_requests enable row level security;

-- No RLS policies = no direct client access. Functions use the service_role key.
