CREATE TABLE idempotency_keys (
    merchant_id      TEXT NOT NULL,
    idempotency_key  TEXT NOT NULL,
    request_hash     TEXT NOT NULL,                  -- SHA-256 do corpo: mesmo key + corpo diferente = 409
    response_status  INTEGER,                        -- NULL enquanto a requisição original ainda roda
    response_body    BYTEA,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at       TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (merchant_id, idempotency_key)
);
CREATE INDEX idempotency_expires_idx ON idempotency_keys (expires_at);

-- Cofre de cartões. É a ÚNICA tabela com PAN, e mesmo assim cifrado (AES-256-GCM).
-- Em produção vive em outro banco, em outra rede, acessível só pelo serviço vault.
CREATE TABLE card_vault (
    token              TEXT PRIMARY KEY,
    pan_ciphertext     BYTEA NOT NULL,
    expiry_ciphertext  BYTEA NOT NULL,
    brand              TEXT NOT NULL,
    bin                TEXT NOT NULL,
    last4              TEXT NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
