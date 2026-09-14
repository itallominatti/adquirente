CREATE TABLE transactions (
    id                  TEXT PRIMARY KEY,
    merchant_id         TEXT NOT NULL REFERENCES merchants (id),
    amount              BIGINT NOT NULL CHECK (amount > 0),   -- centavos
    product             TEXT NOT NULL CHECK (product IN ('DEBIT', 'CREDIT')),
    installments        INTEGER NOT NULL CHECK (installments BETWEEN 1 AND 12),
    status              TEXT NOT NULL,
    card_token          TEXT NOT NULL,
    card_brand          TEXT NOT NULL,
    card_bin            TEXT NOT NULL,
    card_last4          TEXT NOT NULL,
    authorization_code  TEXT NOT NULL DEFAULT '',
    nsu                 TEXT NOT NULL DEFAULT '',
    response_code       TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL,
    authorized_at       TIMESTAMPTZ,
    captured_at         TIMESTAMPTZ,
    canceled_at         TIMESTAMPTZ,
    version             INTEGER NOT NULL DEFAULT 1
);

CREATE INDEX transactions_merchant_created_idx ON transactions (merchant_id, created_at DESC);
CREATE INDEX transactions_status_idx ON transactions (status);

-- NSU: Número Sequencial Único, gerado pelo banco para nunca repetir mesmo com N réplicas da API.
CREATE SEQUENCE nsu_seq START 1;
