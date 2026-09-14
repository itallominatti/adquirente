package cryptoutil

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestAESGCMRoundTrip(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	c, err := NewAESGCM(key)
	if err != nil {
		t.Fatal(err)
	}
	pan := []byte("4111111111111111")
	ct1, _ := c.Encrypt(pan)
	ct2, _ := c.Encrypt(pan)
	if bytes.Equal(ct1, ct2) {
		t.Fatal("nonce aleatório: cifrar duas vezes deve dar resultados diferentes")
	}
	if bytes.Contains(ct1, pan) {
		t.Fatal("o PAN não pode aparecer em claro no ciphertext")
	}
	got, err := c.Decrypt(ct1)
	if err != nil || !bytes.Equal(got, pan) {
		t.Fatalf("decrypt = %q, %v", got, err)
	}
	ct1[len(ct1)-1] ^= 0xFF // adultera um byte
	if _, err := c.Decrypt(ct1); err == nil {
		t.Fatal("GCM deve detectar adulteração")
	}
	if _, err := NewAESGCM(base64.StdEncoding.EncodeToString([]byte("curta"))); err == nil {
		t.Fatal("chave curta deve falhar")
	}
}
