//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/suite"
)

type OnchainDepositRepoSuite struct {
	suite.Suite
	ctx    context.Context
	client *dbent.Client
	repo   *onchainDepositRepository
}

func (s *OnchainDepositRepoSuite) SetupTest() {
	s.ctx = context.Background()
	s.client = testEntClient(s.T())
	s.repo = newOnchainDepositRepositoryWithSQL(s.client, integrationDB)

	_, _ = integrationDB.ExecContext(s.ctx, "DELETE FROM onchain_deposits")
	_, _ = integrationDB.ExecContext(s.ctx, "DELETE FROM onchain_deposit_scan_states")
	_, _ = integrationDB.ExecContext(s.ctx, "DELETE FROM users")
}

func TestOnchainDepositRepoSuite(t *testing.T) {
	suite.Run(t, new(OnchainDepositRepoSuite))
}

func (s *OnchainDepositRepoSuite) mustCreateUser(email string) *service.User {
	userRepo := newUserRepositoryWithSQL(s.client, integrationDB)
	user := &service.User{
		Email:          email,
		PasswordHash:   "test-password-hash",
		Role:           service.RoleUser,
		Status:         service.StatusActive,
		Concurrency:    5,
		BindingAddress: "0x0000000000000000000000000000000000000011",
	}
	s.Require().NoError(userRepo.Create(s.ctx, user))
	return user
}

func (s *OnchainDepositRepoSuite) TestCreateOrGetDetected_DepositIsIdempotent() {
	user := s.mustCreateUser(uniqueTestValue(s.T(), "onchain") + "@example.com")

	first, err := s.repo.CreateOrGetDetected(s.ctx, &service.OnchainDeposit{
		UserID:        user.ID,
		Chain:         "bsc",
		TokenSymbol:   "USDC",
		TokenContract: "0x0000000000000000000000000000000000000001",
		TXHash:        "0xabc",
		LogIndex:      7,
		BlockNumber:   12345,
		BlockHash:     "0xdef",
		FromAddress:   "0x00000000000000000000000000000000000000aa",
		ToAddress:     user.BindingAddress,
		AmountRaw:     "1000000",
		AmountCredit:  1,
		Status:        service.OnchainDepositStatusDetected,
	})
	s.Require().NoError(err)

	second, err := s.repo.CreateOrGetDetected(s.ctx, &service.OnchainDeposit{
		UserID:        user.ID,
		Chain:         "bsc",
		TokenSymbol:   "USDC",
		TokenContract: "0x0000000000000000000000000000000000000001",
		TXHash:        "0xabc",
		LogIndex:      7,
		BlockNumber:   12345,
		BlockHash:     "0xdef",
		FromAddress:   "0x00000000000000000000000000000000000000aa",
		ToAddress:     user.BindingAddress,
		AmountRaw:     "1000000",
		AmountCredit:  1,
		Status:        service.OnchainDepositStatusDetected,
	})
	s.Require().NoError(err)
	s.Require().Equal(first.ID, second.ID)
}

func (s *OnchainDepositRepoSuite) TestUpsertScanState_StoresLatestScannedBlock() {
	err := s.repo.UpsertScanState(s.ctx, "base", 123456)
	s.Require().NoError(err)

	state, err := s.repo.GetScanState(s.ctx, "base")
	s.Require().NoError(err)
	s.Require().Equal("base", state.Chain)
	s.Require().Equal(int64(123456), state.LastScannedBlock)

	err = s.repo.UpsertScanState(s.ctx, "base", 123999)
	s.Require().NoError(err)

	updated, err := s.repo.GetScanState(s.ctx, "base")
	s.Require().NoError(err)
	s.Require().Equal(int64(123999), updated.LastScannedBlock)
}

func (s *OnchainDepositRepoSuite) TestCreditDepositAndBalance_MarksCreditedAndIncreasesUserBalance() {
	user := s.mustCreateUser(uniqueTestValue(s.T(), "credit") + "@example.com")

	deposit, err := s.repo.CreateOrGetDetected(s.ctx, &service.OnchainDeposit{
		UserID:        user.ID,
		Chain:         "arbitrum",
		TokenSymbol:   "USDC",
		TokenContract: "0x0000000000000000000000000000000000000002",
		TXHash:        "0xcredit",
		LogIndex:      3,
		BlockNumber:   22345,
		BlockHash:     "0xblock",
		FromAddress:   "0x00000000000000000000000000000000000000bb",
		ToAddress:     user.BindingAddress,
		AmountRaw:     "2500000",
		AmountCredit:  2.5,
		Status:        service.OnchainDepositStatusDetected,
	})
	s.Require().NoError(err)

	err = s.repo.CreditDepositAndBalance(s.ctx, deposit.ID, user.ID, 2.5)
	s.Require().NoError(err)

	userRepo := newUserRepositoryWithSQL(s.client, integrationDB)
	updatedUser, err := userRepo.GetByID(s.ctx, user.ID)
	s.Require().NoError(err)
	s.Require().Equal(2.5, updatedUser.Balance)

	credited, err := s.repo.GetByID(s.ctx, deposit.ID)
	s.Require().NoError(err)
	s.Require().Equal(service.OnchainDepositStatusCredited, credited.Status)
	s.Require().NotNil(credited.CreditedAt)
}

func (s *OnchainDepositRepoSuite) TestListPendingFailed_ReturnsDetectedAndFailedDeposits() {
	user := s.mustCreateUser(uniqueTestValue(s.T(), "failed") + "@example.com")

	now := time.Now().UTC()
	s.Require().NoError(s.client.OnchainDeposit.Create().
		SetUserID(user.ID).
		SetChain("bsc").
		SetTokenSymbol("USDC").
		SetTokenContract("0x0000000000000000000000000000000000000001").
		SetTxHash("0xf1").
		SetLogIndex(1).
		SetBlockNumber(100).
		SetBlockHash("0xb1").
		SetFromAddress("0x00000000000000000000000000000000000000cc").
		SetToAddress(user.BindingAddress).
		SetAmountRaw("1000000").
		SetAmountCredit(1).
		SetStatus(service.OnchainDepositStatusFailed).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Exec(s.ctx))

	s.Require().NoError(s.client.OnchainDeposit.Create().
		SetUserID(user.ID).
		SetChain("bsc").
		SetTokenSymbol("USDC").
		SetTokenContract("0x0000000000000000000000000000000000000001").
		SetTxHash("0xd1").
		SetLogIndex(2).
		SetBlockNumber(101).
		SetBlockHash("0xb2").
		SetFromAddress("0x00000000000000000000000000000000000000dd").
		SetToAddress(user.BindingAddress).
		SetAmountRaw("2000000").
		SetAmountCredit(2).
		SetStatus(service.OnchainDepositStatusDetected).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Exec(s.ctx))

	deposits, err := s.repo.ListPendingFailed(s.ctx, "bsc", 10)
	s.Require().NoError(err)
	s.Require().Len(deposits, 2)
}
