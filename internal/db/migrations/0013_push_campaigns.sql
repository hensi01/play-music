CREATE TABLE IF NOT EXISTS push_campaigns (
 id TEXT PRIMARY KEY,
 created_by TEXT REFERENCES users(id) ON DELETE SET NULL,
 title TEXT NOT NULL,
 body TEXT NOT NULL,
 url TEXT NOT NULL,
 scheduled_at TIMESTAMPTZ NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 started_at TIMESTAMPTZ,
 finished_at TIMESTAMPTZ,
 status TEXT NOT NULL CHECK (status IN ('pending','sending','completed','partial','unknown','cancelled')),
 selected INTEGER NOT NULL DEFAULT 0,
 queued INTEGER NOT NULL DEFAULT 0,
 detail TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS push_campaigns_due ON push_campaigns(scheduled_at) WHERE status = 'pending';
