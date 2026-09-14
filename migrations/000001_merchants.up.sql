CREATE TABLE merchants (
    id                 TEXT PRIMARY KEY,
    document           TEXT NOT NULL UNIQUE,          -- CNPJ (só dígitos/letras, validado no domínio)
    legal_name         TEXT NOT NULL,
    mcc                TEXT NOT NULL,
    status             TEXT NOT NULL CHECK (status IN ('UNDER_REVIEW', 'ACTIVE', 'BLOCKED')),
    bank_code          TEXT NOT NULL,
    bank_branch        TEXT NOT NULL,
    bank_account       TEXT NOT NULL,
    bank_account_kind  TEXT NOT NULL,
    fee_plan           JSONB NOT NULL,
    webhook_url        TEXT NOT NULL DEFAULT '',
    auto_anticipation  BOOLEAN NOT NULL DEFAULT FALSE,
    version            INTEGER NOT NULL DEFAULT 1,     -- lock otimista
    created_at         TIMESTAMPTZ NOT NULL,
    updated_at         TIMESTAMPTZ NOT NULL
);

CREATE INDEX merchants_status_idx ON merchants (status);
