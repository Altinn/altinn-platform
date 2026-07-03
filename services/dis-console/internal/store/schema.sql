CREATE TABLE IF NOT EXISTS flux_resource (
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
    suspended            boolean     NOT NULL DEFAULT false,
    generation           bigint,
    observed_generation  bigint,
    last_transition      timestamptz,
    raw                  jsonb       NOT NULL,
    content_hash         text,
    first_seen           timestamptz NOT NULL DEFAULT now(),
    last_seen            timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (kind, namespace, name)
);

ALTER TABLE flux_resource ADD COLUMN IF NOT EXISTS content_hash text;
ALTER TABLE flux_resource ADD COLUMN IF NOT EXISTS azure_resource_id text;
ALTER TABLE flux_resource ADD COLUMN IF NOT EXISTS parent_kind text;
ALTER TABLE flux_resource ADD COLUMN IF NOT EXISTS parent_name text;
ALTER TABLE flux_resource ADD COLUMN IF NOT EXISTS applied_by_name text;
ALTER TABLE flux_resource ADD COLUMN IF NOT EXISTS applied_by_namespace text;

CREATE INDEX IF NOT EXISTS idx_flux_resource_ready ON flux_resource (lower(ready));
CREATE INDEX IF NOT EXISTS idx_flux_resource_kind  ON flux_resource (lower(kind));

CREATE TABLE IF NOT EXISTS flux_status_event (
    id          bigserial PRIMARY KEY,
    kind        text NOT NULL,
    namespace   text NOT NULL,
    name        text NOT NULL,
    ready       text NOT NULL,
    reason      text,
    message     text,
    revision    text,
    observed_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_flux_event_obj ON flux_status_event (kind, namespace, name, observed_at DESC);

CREATE TABLE IF NOT EXISTS meta (
    id             boolean     PRIMARY KEY DEFAULT true,
    schema_version integer     NOT NULL,
    agent_version  text        NOT NULL DEFAULT '',
    last_sweep_at  timestamptz,
    CONSTRAINT meta_singleton CHECK (id)
);
