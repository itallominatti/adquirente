package bank

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/itallominatti/adquirente/internal/domain/settlement"
)

type FileGateway struct{ dir string }

var _ settlement.PaymentGateway = (*FileGateway)(nil)

func NewFileGateway(dir string) (*FileGateway, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("criar diretório de liquidação: %w", err)
	}
	return &FileGateway{dir: dir}, nil
}

func (g *FileGateway) Send(_ context.Context, s *settlement.Settlement) (string, error) {
	for _, o := range s.Orders() {
		if strings.HasSuffix(o.BankAccount.Number, "-0") {
			return "", errors.New("conta inválida: " + o.BankAccount.Number)
		}
	}
	name := fmt.Sprintf("settlement-%s-%s.json", s.Date().Format("20060102"), s.ID())
	payload := map[string]any{
		"settlement_id": s.ID(),
		"date":          s.Date().Format("2006-01-02"),
		"merchant_id":   s.MerchantID(),
		"orders":        s.Orders(),
		"total":         s.ToMerchant() + s.ToCreditors(),
	}
	data, _ := json.MarshalIndent(payload, "", "  ")
	if err := os.WriteFile(filepath.Join(g.dir, name), data, 0o640); err != nil {
		return "", fmt.Errorf("gravar arquivo de liquidação: %w", err)
	}
	return name, nil
}
