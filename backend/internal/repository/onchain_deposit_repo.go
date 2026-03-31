package repository

import (
	"context"
	"database/sql"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/onchaindeposit"
	"github.com/Wei-Shaw/sub2api/ent/onchaindepositscanstate"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type onchainDepositRepository struct {
	client *dbent.Client
	sql    *sql.DB
}

func NewOnchainDepositRepository(client *dbent.Client, sqlDB *sql.DB) service.OnchainDepositRepository {
	return newOnchainDepositRepositoryWithSQL(client, sqlDB)
}

func newOnchainDepositRepositoryWithSQL(client *dbent.Client, sqlDB *sql.DB) *onchainDepositRepository {
	return &onchainDepositRepository{client: client, sql: sqlDB}
}

func (r *onchainDepositRepository) GetByID(ctx context.Context, id int64) (*service.OnchainDeposit, error) {
	client := clientFromContext(ctx, r.client)
	entity, err := client.OnchainDeposit.Query().Where(onchaindeposit.IDEQ(id)).Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrUserNotFound, nil)
	}
	return onchainDepositEntityToService(entity), nil
}

func (r *onchainDepositRepository) GetScanState(ctx context.Context, chain string) (*service.OnchainDepositScanState, error) {
	client := clientFromContext(ctx, r.client)
	entity, err := client.OnchainDepositScanState.Query().Where(onchaindepositscanstate.ChainEQ(chain)).Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return &service.OnchainDepositScanState{Chain: chain, LastScannedBlock: 0}, nil
		}
		return nil, err
	}
	return onchainDepositScanStateEntityToService(entity), nil
}

func (r *onchainDepositRepository) UpsertScanState(ctx context.Context, chain string, lastScannedBlock int64) error {
	client := clientFromContext(ctx, r.client)
	existing, err := client.OnchainDepositScanState.Query().Where(onchaindepositscanstate.ChainEQ(chain)).Only(ctx)
	if err != nil {
		if !dbent.IsNotFound(err) {
			return err
		}
		_, err = client.OnchainDepositScanState.Create().
			SetChain(chain).
			SetLastScannedBlock(lastScannedBlock).
			Save(ctx)
		return err
	}
	_, err = client.OnchainDepositScanState.UpdateOneID(existing.ID).
		SetLastScannedBlock(lastScannedBlock).
		Save(ctx)
	return err
}

func (r *onchainDepositRepository) CreateOrGetDetected(ctx context.Context, deposit *service.OnchainDeposit) (*service.OnchainDeposit, error) {
	if deposit == nil {
		return nil, nil
	}
	client := clientFromContext(ctx, r.client)

	created, err := client.OnchainDeposit.Create().
		SetUserID(deposit.UserID).
		SetChain(deposit.Chain).
		SetTokenSymbol(deposit.TokenSymbol).
		SetTokenContract(deposit.TokenContract).
		SetTxHash(deposit.TXHash).
		SetLogIndex(deposit.LogIndex).
		SetBlockNumber(deposit.BlockNumber).
		SetBlockHash(deposit.BlockHash).
		SetFromAddress(deposit.FromAddress).
		SetToAddress(deposit.ToAddress).
		SetAmountRaw(deposit.AmountRaw).
		SetAmountCredit(deposit.AmountCredit).
		SetStatus(deposit.Status).
		Save(ctx)
	if err == nil {
		return onchainDepositEntityToService(created), nil
	}
	if !isUniqueConstraintViolation(err) {
		return nil, err
	}

	existing, getErr := client.OnchainDeposit.Query().
		Where(
			onchaindeposit.ChainEQ(deposit.Chain),
			onchaindeposit.TxHashEQ(deposit.TXHash),
			onchaindeposit.LogIndexEQ(deposit.LogIndex),
		).
		Only(ctx)
	if getErr != nil {
		return nil, getErr
	}
	return onchainDepositEntityToService(existing), nil
}

func (r *onchainDepositRepository) CreditDepositAndBalance(ctx context.Context, depositID int64, userID int64, amount float64) error {
	tx := dbent.TxFromContext(ctx)
	if tx != nil {
		return r.creditDepositAndBalanceWithClient(ctx, tx.Client(), depositID, userID, amount)
	}

	startedTx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = startedTx.Rollback() }()

	txCtx := dbent.NewTxContext(ctx, startedTx)
	if err := r.creditDepositAndBalanceWithClient(txCtx, startedTx.Client(), depositID, userID, amount); err != nil {
		return err
	}
	return startedTx.Commit()
}

func (r *onchainDepositRepository) creditDepositAndBalanceWithClient(ctx context.Context, client *dbent.Client, depositID int64, userID int64, amount float64) error {
	now := time.Now().UTC()
	updated, err := client.OnchainDeposit.Update().
		Where(
			onchaindeposit.IDEQ(depositID),
			onchaindeposit.StatusNEQ(service.OnchainDepositStatusCredited),
		).
		SetStatus(service.OnchainDepositStatusCredited).
		SetCreditedAt(now).
		ClearErrorMessage().
		Save(ctx)
	if err != nil {
		return err
	}
	if updated == 0 {
		return nil
	}

	if _, err := client.User.UpdateOneID(userID).AddBalance(amount).Save(ctx); err != nil {
		return err
	}
	return nil
}

func (r *onchainDepositRepository) ListPendingFailed(ctx context.Context, chain string, limit int) ([]service.OnchainDeposit, error) {
	if limit <= 0 {
		limit = 100
	}
	client := clientFromContext(ctx, r.client)
	entities, err := client.OnchainDeposit.Query().
		Where(
			onchaindeposit.ChainEQ(chain),
			onchaindeposit.StatusIn(service.OnchainDepositStatusDetected, service.OnchainDepositStatusFailed),
		).
		Order(
			dbent.Asc(onchaindeposit.FieldBlockNumber),
			dbent.Asc(onchaindeposit.FieldLogIndex),
		).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.OnchainDeposit, 0, len(entities))
	for _, entity := range entities {
		out = append(out, *onchainDepositEntityToService(entity))
	}
	return out, nil
}

func onchainDepositEntityToService(entity *dbent.OnchainDeposit) *service.OnchainDeposit {
	if entity == nil {
		return nil
	}
	return &service.OnchainDeposit{
		ID:            entity.ID,
		UserID:        entity.UserID,
		Chain:         entity.Chain,
		TokenSymbol:   entity.TokenSymbol,
		TokenContract: entity.TokenContract,
		TXHash:        entity.TxHash,
		LogIndex:      entity.LogIndex,
		BlockNumber:   entity.BlockNumber,
		BlockHash:     entity.BlockHash,
		FromAddress:   entity.FromAddress,
		ToAddress:     entity.ToAddress,
		AmountRaw:     entity.AmountRaw,
		AmountCredit:  entity.AmountCredit,
		Status:        entity.Status,
		CreditedAt:    entity.CreditedAt,
		ErrorMessage:  entity.ErrorMessage,
		CreatedAt:     entity.CreatedAt,
		UpdatedAt:     entity.UpdatedAt,
	}
}

func onchainDepositScanStateEntityToService(entity *dbent.OnchainDepositScanState) *service.OnchainDepositScanState {
	if entity == nil {
		return nil
	}
	return &service.OnchainDepositScanState{
		ID:               entity.ID,
		Chain:            entity.Chain,
		LastScannedBlock: entity.LastScannedBlock,
		CreatedAt:        entity.CreatedAt,
		UpdatedAt:        entity.UpdatedAt,
	}
}
