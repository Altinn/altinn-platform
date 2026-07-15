CREATE TABLE IF NOT EXISTS flux_resource (
    cluster              text        NOT NULL,
    kind                 text        NOT NULL,
    api_version          text        NOT NULL,
    namespace            text        NOT NULL,
    name                 text        NOT NULL,
    ready                text        NOT NULL DEFAULT 'Unknown',
    reason               text,
    message              text,
    revision             text,
    azure_resource_id    text,
    parent_kind          text,
    parent_name          text,
    applied_by_name      text,
    applied_by_namespace text,
    source_ref_kind      text,
    source_ref_name      text,
    source_ref_namespace text,
    source_url           text,
    origin_revision      text,
    origin_source        text,
    inventory            jsonb,
    suspended            boolean     NOT NULL DEFAULT false,
    generation           bigint,
    observed_generation  bigint,
    last_transition      timestamptz,
    raw                  jsonb       NOT NULL,
    content_hash         text,
    updated_at           timestamptz NOT NULL,
    synced_at            timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (cluster, kind, namespace, name)
);

CREATE INDEX IF NOT EXISTS idx_central_resource_ready   ON flux_resource (lower(ready));
CREATE INDEX IF NOT EXISTS idx_central_resource_kind    ON flux_resource (lower(kind));
CREATE INDEX IF NOT EXISTS idx_central_resource_cluster ON flux_resource (cluster);

CREATE TABLE IF NOT EXISTS flux_status_event (
    cluster     text        NOT NULL,
    tenant_id   bigint      NOT NULL,
    kind        text        NOT NULL,
    namespace   text        NOT NULL,
    name        text        NOT NULL,
    ready       text        NOT NULL,
    reason      text,
    message     text,
    revision    text,
    observed_at timestamptz NOT NULL,
    PRIMARY KEY (cluster, tenant_id)
);

CREATE INDEX IF NOT EXISTS idx_central_event_obj ON flux_status_event (cluster, lower(kind), namespace, name, observed_at DESC, tenant_id DESC);
CREATE INDEX IF NOT EXISTS idx_central_event_observed_at ON flux_status_event (observed_at);

CREATE TABLE IF NOT EXISTS cluster_report (
    cluster        text        PRIMARY KEY,
    environment    text        NOT NULL DEFAULT '',
    sync_cursor    timestamptz,
    event_cursor   bigint      NOT NULL DEFAULT 0,
    last_synced_at timestamptz NOT NULL DEFAULT now(),
    last_sweep_at  timestamptz,
    agent_version  text        NOT NULL DEFAULT '',
    schema_version integer     NOT NULL DEFAULT 0,
    resource_count integer     NOT NULL DEFAULT 0
);

ALTER TABLE cluster_report ADD COLUMN IF NOT EXISTS event_cursor bigint NOT NULL DEFAULT 0;

ALTER TABLE flux_resource ADD COLUMN IF NOT EXISTS azure_resource_id text;
ALTER TABLE flux_resource ADD COLUMN IF NOT EXISTS parent_kind text;
ALTER TABLE flux_resource ADD COLUMN IF NOT EXISTS parent_name text;
ALTER TABLE flux_resource ADD COLUMN IF NOT EXISTS applied_by_name text;
ALTER TABLE flux_resource ADD COLUMN IF NOT EXISTS applied_by_namespace text;
ALTER TABLE flux_resource ADD COLUMN IF NOT EXISTS source_ref_kind text;
ALTER TABLE flux_resource ADD COLUMN IF NOT EXISTS source_ref_name text;
ALTER TABLE flux_resource ADD COLUMN IF NOT EXISTS source_ref_namespace text;
ALTER TABLE flux_resource ADD COLUMN IF NOT EXISTS source_url text;
ALTER TABLE flux_resource ADD COLUMN IF NOT EXISTS origin_revision text;
ALTER TABLE flux_resource ADD COLUMN IF NOT EXISTS origin_source text;
ALTER TABLE flux_resource ADD COLUMN IF NOT EXISTS inventory jsonb;
