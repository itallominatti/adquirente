CREATE TABLE ledger_entries (
    id              TEXT PRIMARY KEY,
    account         TEXT NOT NULL,
    merchant_id     TEXT NOT NULL,
    debit           BIGINT NOT NULL DEFAULT 0 CHECK (debit >= 0),
    credit          BIGINT NOT NULL DEFAULT 0 CHECK (credit >= 0),
    reference_type  TEXT NOT NULL,
    reference_id    TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL,
    CHECK (debit = 0 OR credit = 0)
);
CREATE INDEX ledger_account_merchant_idx ON ledger_entries (account, merchant_id);
CREATE INDEX ledger_reference_idx        ON ledger_entries (reference_type, reference_id);

-- Outbox: eventos gravados na MESMA transação do dado; o relay publica no Kafka depois.
CREATE TABLE outbox (
    id            BIGSERIAL PRIMARY KEY,           -- ordem de publicação
    event_id      TEXT NOT NULL UNIQUE,
    aggregate_id  TEXT NOT NULL,
    event_type    TEXT NOT NULL,
    payload       JSONB NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at  TIMESTAMPTZ
);
CREATE INDEX outbox_unpublished_idx ON outbox (id) WHERE published_at IS NULL;

-- Consumidores idempotentes: (consumidor, evento) só processa uma vez.
CREATE TABLE processed_events (
    consumer      TEXT NOT NULL,
    event_id      TEXT NOT NULL,
    processed_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (consumer, event_id)
);
