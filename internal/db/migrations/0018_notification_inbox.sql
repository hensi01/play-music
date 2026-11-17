CREATE TABLE notification_inbox (
 id TEXT PRIMARY KEY,
 title TEXT NOT NULL,
 body TEXT NOT NULL,
 url TEXT NOT NULL,
 published_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX notification_inbox_published ON notification_inbox(published_at DESC, id DESC);
INSERT INTO notification_inbox(id,title,body,url,published_at) SELECT id,title,body,url,COALESCE(finished_at,scheduled_at) FROM push_campaigns WHERE status='completed';
