CREATE TABLE settlements (
    id               TEXT PRIMARY KEY,
    settlement_date  DATE NOT NULL,
    merchant_id      TEXT NOT NULL REFERENCES merchants (id),
    total_amount     BIGINT NOT NULL,
    to_merchant      BIGINT NOT NULL,
    to_creditors     BIGINT NOT NULL,
    retained         BIGINT NOT NULL,
    orders           JSONB NOT NULL,
    receivable_ids   JSONB NOT NULL,
    status           TEXT NOT NULL CHECK (status IN ('PENDING', 'SENT', 'CONFIRMED', 'FAILED')),
    external_ref     TEXT NOT NULL DEFAULT '',
    failure_reason   TEXT NOT NULL DEFAULT '',
    version          INTEGER NOT NULL DEFAULT 1,
    created_at       TIMESTAMPTZ NOT NULL,
    updated_at       TIMESTAMPTZ NOT NULL
);
CREATE INDEX settlements_date_idx     ON settlements (settlement_date);
CREATE INDEX settlements_merchant_idx ON settlements (merchant_id, settlement_date DESC);

CREATE TABLE anticipations (
    id               TEXT PRIMARY KEY,
    merchant_id      TEXT NOT NULL REFERENCES merchants (id),
    items            JSONB NOT NULL,
    gross_amount     BIGINT NOT NULL,
    discount_amount  BIGINT NOT NULL,
    net_amount       BIGINT NOT NULL,
    rate_bps_month   INTEGER NOT NULL,
    requested_at     TIMESTAMPTZ NOT NULL,
    status           TEXT NOT NULL CHECK (status IN ('SIMULATED', 'CONFIRMED')),
    version          INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX anticipations_merchant_idx ON anticipations (merchant_id, requested_at DESC);

CREATE TABLE chargebacks (
    id               TEXT PRIMARY KEY,
    transaction_id   TEXT NOT NULL REFERENCES transactions (id),
    merchant_id      TEXT NOT NULL REFERENCES merchants (id),
    reason_code      TEXT NOT NULL,
    amount           BIGINT NOT NULL,
    status           TEXT NOT NULL CHECK (status IN ('OPENED', 'DEFENDED', 'ACCEPTED', 'REVERSED')),
    opened_at        TIMESTAMPTZ NOT NULL,
    deadline         TIMESTAMPTZ NOT NULL,
    resolved_at      TIMESTAMPTZ,
    version          INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX chargebacks_transaction_idx ON chargebacks (transaction_id);
