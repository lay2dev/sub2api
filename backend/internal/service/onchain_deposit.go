package service

import (
	"context"
	"time"
)

const (
	OnchainDepositStatusDetected = "detected"
	OnchainDepositStatusCredited = "credited"
	OnchainDepositStatusFailed   = "failed"
)

type OnchainDeposit struct {
	ID            int64
	UserID        int64
	Chain         string
	TokenSymbol   string
	TokenContract string
	TXHash        string
	LogIndex      int64
	BlockNumber   int64
	BlockHash     string
	FromAddress   string
	ToAddress     string
	AmountRaw     string
	AmountCredit  float64
	Status        string
	CreditedAt    *time.Time
	ErrorMessage  *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type OnchainDepositScanState struct {
	ID               int64
	Chain            string
	LastScannedBlock int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type OnchainDepositRepository interface {
	GetByID(ctx context.Context, id int64) (*OnchainDeposit, error)
	GetScanState(ctx context.Context, chain string) (*OnchainDepositScanState, error)
	UpsertScanState(ctx context.Context, chain string, lastScannedBlock int64) error
	CreateOrGetDetected(ctx context.Context, deposit *OnchainDeposit) (*OnchainDeposit, error)
	CreditDepositAndBalance(ctx context.Context, depositID int64, userID int64, amount float64) error
	ListPendingFailed(ctx context.Context, chain string, limit int) ([]OnchainDeposit, error)
}
