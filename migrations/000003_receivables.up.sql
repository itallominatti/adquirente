CREATE TABLE receivables (
    id               TEXT PRIMARY KEY,
    transaction_id   TEXT NOT NULL REFERENCES transactions (id),
    merchant_id      TEXT NOT NULL REFERENCES merchants (id),
    installment_no   INTEGER NOT NULL,
    installments     INTEGER NOT NULL,
    product          TEXT NOT NULL,
    brand            TEXT NOT NULL,
    gross_amount     BIGINT NOT NULL,
    fee_amount       BIGINT NOT NULL,
    net_amount       BIGINT NOT NULL,
    due_date         DATE NOT NULL,
    status           TEXT NOT NULL CHECK (status IN ('SCHEDULED', 'ANTICIPATED', 'SETTLED', 'CANCELED', 'CHARGEBACKED')),
    settlement_id    TEXT NOT NULL DEFAULT '',
    anticipation_id  TEXT NOT NULL DEFAULT '',
    version          INTEGER NOT NULL DEFAULT 1,
    created_at       TIMESTAMPTZ NOT NULL,
    updated_at       TIMESTAMPTZ NOT NULL
);

-- As consultas que o sistema faz de verdade:
CREATE INDEX receivables_due_status_idx      ON receivables (due_date, status);            -- worker de liquidação
CREATE INDEX receivables_merchant_due_idx    ON receivables (merchant_id, due_date);       -- agenda do EC
CREATE INDEX receivables_transaction_idx     ON receivables (transaction_id);              -- cancelamento/chargeback
