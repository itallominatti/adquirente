package gormrepo

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/itallominatti/adquirente/internal/domain/merchant"
	"github.com/itallominatti/adquirente/internal/domain/shared"
)

// Este pacote mostra um ORM (GORM) no cadastro, para você comparar com o SQL explícito do núcleo.
// Regra: o Model é um detalhe de infraestrutura. Ele NUNCA sai deste pacote; o domínio só vê merchant.Merchant.

type feePlanJSON struct {
	Debit              int         `json:"debit_bps"`
	CreditOneShot      int         `json:"credit_bps"`
	CreditInstallments map[int]int `json:"credit_installments_bps"`
	AnticipationMonth  int         `json:"anticipation_bps_month"`
}

func (f *feePlanJSON) Scan(src any) error {
	b, ok := src.([]byte)
	if !ok {
		if s, ok := src.(string); ok {
			b = []byte(s)
		} else {
			return fmt.Errorf("fee_plan: tipo inesperado %T", src)
		}
	}
	return json.Unmarshal(b, f)
}

func (f feePlanJSON) Value() (driver.Value, error) { return json.Marshal(f) }

type MerchantModel struct {
	ID               string      `gorm:"primaryKey"`
	Document         string      `gorm:"uniqueIndex;not null"`
	LegalName        string      `gorm:"not null"`
	MCC              string      `gorm:"column:mcc;not null"`
	Status           string      `gorm:"not null;index"`
	BankCode         string      `gorm:"not null"`
	BankBranch       string      `gorm:"not null"`
	BankAccount      string      `gorm:"not null"`
	BankAccountKind  string      `gorm:"not null"`
	FeePlan          feePlanJSON `gorm:"type:jsonb;not null"`
	WebhookURL       string      `gorm:"column:webhook_url;not null"`
	AutoAnticipation bool        `gorm:"not null"`
	Version          int         `gorm:"not null"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (MerchantModel) TableName() string { return "merchants" }

type MerchantRepository struct{ db *gorm.DB }

func New(sqlDB *sql.DB) (*MerchantRepository, error) {
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, fmt.Errorf("abrir gorm: %w", err)
	}
	return &MerchantRepository{db: db}, nil
}

var _ merchant.Repository = (*MerchantRepository)(nil)

func (r *MerchantRepository) Create(ctx context.Context, m *merchant.Merchant) error {
	if err := r.db.WithContext(ctx).Create(toModel(m)).Error; err != nil {
		return fmt.Errorf("inserir estabelecimento: %w", err)
	}
	return nil
}

func (r *MerchantRepository) FindByID(ctx context.Context, id string) (*merchant.Merchant, error) {
	var model MerchantModel
	err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, shared.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("buscar estabelecimento: %w", err)
	}
	return toDomain(model)
}

func (r *MerchantRepository) Update(ctx context.Context, m *merchant.Merchant) error {
	model := toModel(m)
	res := r.db.WithContext(ctx).Model(&MerchantModel{}).
		Where("id = ? AND version < ?", model.ID, model.Version).
		Select("*").Omit("id", "created_at").Updates(model)
	if res.Error != nil {
		return fmt.Errorf("atualizar estabelecimento: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return shared.ErrConcurrentUpdate
	}
	return nil
}

func toModel(m *merchant.Merchant) *MerchantModel {
	plan := m.FeePlan()
	inst := map[int]int{}
	for n, bps := range plan.CreditInstallments() {
		inst[n] = int(bps)
	}
	b := m.BankAccount()
	return &MerchantModel{
		ID: m.ID(), Document: m.Document().String(), LegalName: m.LegalName(), MCC: m.MCC(), Status: string(m.Status()),
		BankCode: b.BankCode, BankBranch: b.Branch, BankAccount: b.Number, BankAccountKind: string(b.Kind),
		FeePlan:    feePlanJSON{Debit: int(plan.Debit()), CreditOneShot: int(plan.CreditOneShot()), CreditInstallments: inst, AnticipationMonth: int(plan.AnticipationRateMonth())},
		WebhookURL: m.WebhookURL(), AutoAnticipation: m.AutoAnticipation(), Version: m.Version(), CreatedAt: m.CreatedAt(), UpdatedAt: m.UpdatedAt(),
	}
}

func toDomain(model MerchantModel) (*merchant.Merchant, error) {
	inst := map[int]shared.Bps{}
	for n, bps := range model.FeePlan.CreditInstallments {
		inst[n] = shared.Bps(bps)
	}
	plan, err := merchant.NewFeePlan(shared.Bps(model.FeePlan.Debit), shared.Bps(model.FeePlan.CreditOneShot), inst, shared.Bps(model.FeePlan.AnticipationMonth))
	if err != nil {
		return nil, err
	}
	bank := merchant.BankAccount{BankCode: model.BankCode, Branch: model.BankBranch, Number: model.BankAccount, Kind: merchant.AccountKind(model.BankAccountKind)}
	return merchant.Restore(model.ID, merchant.Document(model.Document), model.LegalName, model.MCC, merchant.Status(model.Status), bank, plan,
		model.WebhookURL, model.AutoAnticipation, model.Version, model.CreatedAt, model.UpdatedAt), nil
}
