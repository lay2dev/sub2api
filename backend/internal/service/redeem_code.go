package service

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

type RedeemCode struct {
	ID        int64
	Code      string
	Type      string
	Value     float64
	Status    string
	UsedBy    *int64
	UsedAt    *time.Time
	MaxUses   int
	UsedCount int
	Notes     string
	CreatedAt time.Time

	GroupID      *int64
	ValidityDays int

	User         *User
	Group        *Group
	IssuedAPIKey *APIKey

	UsageRecords []RedeemCodeUsage
}

type RedeemCodeUsage struct {
	ID           int64
	RedeemCodeID int64
	UserID       int64
	APIKeyID     int64
	UsedAt       time.Time

	RedeemCode *RedeemCode
	User       *User
	APIKey     *APIKey
}

func (r *RedeemCode) IsUsed() bool {
	return r.Status == StatusUsed
}

func (r *RedeemCode) CanUse() bool {
	return r.Status == StatusUnused
}

func GenerateRedeemCode() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
