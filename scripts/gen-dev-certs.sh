set -e
mkdir -p certs
cd certs


openssl genrsa -out jwt-private.pem 2048 2>/dev/null
openssl rsa -in jwt-private.pem -pubout -out jwt-public.pem 2>/dev/null

cat > clients.json <<'JSON'
{
  "operador": {
    "secret": "op-s3cr3t",
    "scope": "merchants:read merchants:write merchants:approve chargebacks:write"
  },
  "client_padaria": {
    "secret": "padaria-s3cr3t",
    "merchant_id": "SUBSTITUA_PELO_ID_DO_EC",
    "scope": "merchants:read merchants:write transactions:write transactions:read receivables:read anticipations:write"
  }
}
JSON

# Ônus de exemplo para a registradora simulada (vazio = sem ônus)
echo "[]" > liens.json

# Chave AES-256 do vault (32 bytes em base64) → cole no .env como VAULT_KEY_BASE64
echo "VAULT_KEY_BASE64=$(openssl rand -base64 32)" > vault.env

echo "ok: certs/jwt-*.pem, certs/clients.json, certs/liens.json, certs/vault.env"
