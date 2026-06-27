CREATE TABLE IF NOT EXISTS platform_resources (
    tenant_id TEXT NOT NULL DEFAULT 'default',
    kind TEXT NOT NULL,
    id TEXT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, kind, id)
);

CREATE INDEX IF NOT EXISTS platform_resources_kind_created_idx
    ON platform_resources (tenant_id, kind, created_at DESC);

CREATE TABLE IF NOT EXISTS audit_events (
    sequence BIGSERIAL PRIMARY KEY,
    tenant_id TEXT NOT NULL DEFAULT 'default',
    id TEXT NOT NULL UNIQUE,
    action TEXT NOT NULL,
    resource TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    actor TEXT NOT NULL,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS outbox_events (
    sequence BIGSERIAL PRIMARY KEY,
    tenant_id TEXT NOT NULL DEFAULT 'default',
    id TEXT NOT NULL UNIQUE,
    topic TEXT NOT NULL,
    event_key TEXT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT
);

CREATE INDEX IF NOT EXISTS outbox_unpublished_idx
    ON outbox_events (sequence) WHERE published_at IS NULL;

ALTER TABLE platform_resources ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE outbox_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform_resources FORCE ROW LEVEL SECURITY;
ALTER TABLE audit_events FORCE ROW LEVEL SECURITY;
ALTER TABLE outbox_events FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation_resources ON platform_resources;
CREATE POLICY tenant_isolation_resources ON platform_resources
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));

DROP POLICY IF EXISTS tenant_isolation_audit ON audit_events;
CREATE POLICY tenant_isolation_audit ON audit_events
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));

DROP POLICY IF EXISTS tenant_isolation_outbox ON outbox_events;
CREATE POLICY tenant_isolation_outbox ON outbox_events
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
