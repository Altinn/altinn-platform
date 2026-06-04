CREATE TABLE IF NOT EXISTS flux_resource (
    kind                text        NOT NULL,
    api_version         text        NOT NULL,
    namespace           text        NOT NULL,
    name                text        NOT NULL,
    ready               text        NOT NULL DEFAULT 'Unknown',
    reason              text,
    message             text,
    revision            text,
    suspended           boolean     NOT NULL DEFAULT false,
    generation          bigint,
    observed_generation bigint,
    last_transition     timestamptz,
    raw                 jsonb       NOT NULL,
    first_seen          timestamptz NOT NULL DEFAULT now(),
    last_seen           timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (kind, namespace, name)
);

CREATE INDEX IF NOT EXISTS idx_flux_resource_ready ON flux_resource (ready);
CREATE INDEX IF NOT EXISTS idx_flux_resource_kind  ON flux_resource (kind);

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
