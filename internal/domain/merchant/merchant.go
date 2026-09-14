package merchant

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/itallominatti/adquirente/internal/domain/shared"
)

type Status string

const (
	UnderReview Status = "UNDER_REVIEW"
	Active      Status = "ACTIVE"
	Blocked     Status = "BLOCKED"
)

var (
	ErrInvalidLegalName = errors.New("a razão social é obrigatória")
	ErrInvalidMCC       = errors.New("o MCC deve ter 4 dígitos")
	ErrMerchantInactive = errors.New("estabelecimento não está ativo")
	ErrInvalidWebhook   = errors.New("URL de webhook inválida: use https e um host público")
)

type Merchant struct {
	id               string
	document         Document
	legalName        string
	mcc              string
	status           Status
	bankAccount      BankAccount
	feePlan          FeePlan
	webhookURL       string
	autoAnticipation bool
	version          int
	createdAt        time.Time
	updatedAt        time.Time
}

func New(document Document, legalName, mcc string, bank BankAccount, plan FeePlan, now time.Time) (*Merchant, error) {
	legalName = strings.TrimSpace(legalName)
	if legalName == "" {
		return nil, ErrInvalidLegalName
	}
	if len(mcc) != 4 {
		return nil, ErrInvalidMCC
	}
	if bank.IsZero() {
		return nil, ErrInvalidBankAccount
	}
	return &Merchant{
		id:          shared.NewID("m"),
		document:    document,
		legalName:   legalName,
		mcc:         mcc,
		status:      UnderReview,
		bankAccount: bank,
		feePlan:     plan,
		version:     1,
		createdAt:   now,
		updatedAt:   now,
	}, nil
}

func Restore(id string, document Document, legalName, mcc string, status Status, bank BankAccount, plan FeePlan,
	webhookURL string, autoAnticipation bool, version int, createdAt, updatedAt time.Time) *Merchant {
	return &Merchant{
		id: id, document: document, legalName: legalName, mcc: mcc, status: status, bankAccount: bank,
		feePlan: plan, webhookURL: webhookURL, autoAnticipation: autoAnticipation, version: version,
		createdAt: createdAt, updatedAt: updatedAt,
	}
}

func (m *Merchant) Approve(now time.Time) error {
	if m.status != UnderReview {
		return fmt.Errorf("%w: %s → %s", shared.ErrInvalidTransition, m.status, Active)
	}
	m.status = Active
	m.touch(now)
	return nil
}

func (m *Merchant) Block(now time.Time) error {
	if m.status != Active {
		return fmt.Errorf("%w: %s → %s", shared.ErrInvalidTransition, m.status, Blocked)
	}
	m.status = Blocked
	m.touch(now)
	return nil
}

func (m *Merchant) SetFeePlan(plan FeePlan, now time.Time) {
	m.feePlan = plan
	m.touch(now)
}

func (m *Merchant) SetWebhook(raw string, now time.Time) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return ErrInvalidWebhook
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") ||
		strings.HasPrefix(host, "10.") || strings.HasPrefix(host, "192.168.") || strings.HasPrefix(host, "127.") ||
		strings.HasPrefix(host, "169.254.") || strings.HasPrefix(host, "172.") {
		return ErrInvalidWebhook
	}
	m.webhookURL = raw
	m.touch(now)
	return nil
}

func (m *Merchant) EnableAutoAnticipation(on bool, now time.Time) {
	m.autoAnticipation = on
	m.touch(now)
}

func (m *Merchant) touch(now time.Time) {
	m.updatedAt = now
	m.version++
}

func (m *Merchant) IsActive() bool { return m.status == Active }

func (m *Merchant) ID() string               { return m.id }
func (m *Merchant) Document() Document       { return m.document }
func (m *Merchant) LegalName() string        { return m.legalName }
func (m *Merchant) MCC() string              { return m.mcc }
func (m *Merchant) Status() Status           { return m.status }
func (m *Merchant) BankAccount() BankAccount { return m.bankAccount }
func (m *Merchant) FeePlan() FeePlan         { return m.feePlan }
func (m *Merchant) WebhookURL() string       { return m.webhookURL }
func (m *Merchant) AutoAnticipation() bool   { return m.autoAnticipation }
func (m *Merchant) Version() int             { return m.version }
func (m *Merchant) CreatedAt() time.Time     { return m.createdAt }
func (m *Merchant) UpdatedAt() time.Time     { return m.updatedAt }
