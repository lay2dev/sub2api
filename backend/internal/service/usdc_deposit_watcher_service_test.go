//go:build unit

package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubDepositUserResolver struct {
	usersByAddress map[string]int64
}

func (s *stubDepositUserResolver) ResolveUserIDByAddress(_ context.Context, address string) (int64, bool, error) {
	userID, ok := s.usersByAddress[strings.ToLower(address)]
	return userID, ok, nil
}

func (s *stubDepositUserResolver) ListBindingAddresses(_ context.Context) ([]string, error) {
	if len(s.usersByAddress) == 0 {
		return nil, nil
	}
	addrs := make([]string, 0, len(s.usersByAddress))
	for addr := range s.usersByAddress {
		addrs = append(addrs, addr)
	}
	return addrs, nil
}

type stubOnchainDepositRepo struct {
	scanStates       map[string]int64
	depositsByLogKey map[string]*OnchainDeposit
	nextID           int64
	creditCalls      map[int64]creditCall
	upsertErr        error
}

type creditCall struct {
	userID int64
	amount float64
}

func newStubOnchainDepositRepo() *stubOnchainDepositRepo {
	return &stubOnchainDepositRepo{
		scanStates:       map[string]int64{},
		depositsByLogKey: map[string]*OnchainDeposit{},
		nextID:           1,
		creditCalls:      map[int64]creditCall{},
	}
}

func (r *stubOnchainDepositRepo) GetByID(_ context.Context, id int64) (*OnchainDeposit, error) {
	for _, d := range r.depositsByLogKey {
		if d.ID == id {
			clone := *d
			return &clone, nil
		}
	}
	return nil, errors.New("not found")
}

func (r *stubOnchainDepositRepo) GetScanState(_ context.Context, chain string) (*OnchainDepositScanState, error) {
	return &OnchainDepositScanState{
		Chain:            chain,
		LastScannedBlock: r.scanStates[chain],
	}, nil
}

func (r *stubOnchainDepositRepo) UpsertScanState(_ context.Context, chain string, lastScannedBlock int64) error {
	if r.upsertErr != nil {
		return r.upsertErr
	}
	r.scanStates[chain] = lastScannedBlock
	return nil
}

func (r *stubOnchainDepositRepo) CreateOrGetDetected(_ context.Context, deposit *OnchainDeposit) (*OnchainDeposit, error) {
	key := fmt.Sprintf("%s:%s:%d", strings.ToLower(deposit.Chain), strings.ToLower(deposit.TXHash), deposit.LogIndex)
	if existing, ok := r.depositsByLogKey[key]; ok {
		clone := *existing
		return &clone, nil
	}

	clone := *deposit
	clone.ID = r.nextID
	r.nextID++
	r.depositsByLogKey[key] = &clone
	return &clone, nil
}

func (r *stubOnchainDepositRepo) CreditDepositAndBalance(_ context.Context, depositID int64, userID int64, amount float64) error {
	if _, ok := r.creditCalls[depositID]; ok {
		return nil
	}
	r.creditCalls[depositID] = creditCall{userID: userID, amount: amount}
	for _, d := range r.depositsByLogKey {
		if d.ID == depositID {
			d.Status = OnchainDepositStatusCredited
			return nil
		}
	}
	return nil
}

func (r *stubOnchainDepositRepo) ListPendingFailed(_ context.Context, _ string, _ int) ([]OnchainDeposit, error) {
	return nil, nil
}

func (r *stubOnchainDepositRepo) scanState(chain string) int64 {
	return r.scanStates[chain]
}

func (r *stubOnchainDepositRepo) creditedCount() int {
	return len(r.creditCalls)
}

func (r *stubOnchainDepositRepo) singleCreditCall(t *testing.T) creditCall {
	t.Helper()
	require.Len(t, r.creditCalls, 1)
	for _, call := range r.creditCalls {
		return call
	}
	t.Fatalf("expected one credit call")
	return creditCall{}
}

type stubEVMRPCClient struct {
	latest  uint64
	logs    []EVMTransferLog
	err     error
	filters []EVMTransferLogFilter
}

func (s *stubEVMRPCClient) LatestBlock(_ context.Context, _ string) (uint64, error) {
	if s.err != nil {
		return 0, s.err
	}
	return s.latest, nil
}

func (s *stubEVMRPCClient) GetERC20TransferLogs(_ context.Context, req EVMTransferLogFilter) ([]EVMTransferLog, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.filters = append(s.filters, req)
	toAddressFilterSet := make(map[string]struct{}, len(req.ToAddresses))
	for _, addr := range req.ToAddresses {
		toAddressFilterSet[strings.ToLower(addr)] = struct{}{}
	}

	var out []EVMTransferLog
	for _, log := range s.logs {
		if !strings.EqualFold(log.Chain, req.Chain) {
			continue
		}
		if req.Contract != "" && !strings.EqualFold(log.Contract, req.Contract) {
			continue
		}
		if len(toAddressFilterSet) > 0 {
			if _, ok := toAddressFilterSet[strings.ToLower(log.ToAddress)]; !ok {
				continue
			}
		}
		if log.BlockNumber < req.FromBlock || log.BlockNumber > req.ToBlock {
			continue
		}
		out = append(out, log)
	}
	return out, nil
}

func TestWatcher_CreditsMatchedConfirmedTransferOnce(t *testing.T) {
	repo := newStubOnchainDepositRepo()
	repo.scanStates["base"] = 100

	rpc := &stubEVMRPCClient{
		latest: 120,
		logs: []EVMTransferLog{
			{
				Chain:       "base",
				Contract:    "0x0000000000000000000000000000000000000usd",
				BlockNumber: 112,
				BlockHash:   "0xblock",
				TXHash:      "0xtx1",
				LogIndex:    1,
				FromAddress: "0x0000000000000000000000000000000000000aaa",
				ToAddress:   "0x0000000000000000000000000000000000000011",
				ValueRaw:    "1000000",
			},
			{
				Chain:       "base",
				Contract:    "0x0000000000000000000000000000000000000usd",
				BlockNumber: 112,
				BlockHash:   "0xblock",
				TXHash:      "0xtx2",
				LogIndex:    2,
				FromAddress: "0x0000000000000000000000000000000000000bbb",
				ToAddress:   "0x0000000000000000000000000000000000000099",
				ValueRaw:    "2000000",
			},
			{
				Chain:       "base",
				Contract:    "0x0000000000000000000000000000000000000bad",
				BlockNumber: 112,
				BlockHash:   "0xblock",
				TXHash:      "0xtx-contract-mismatch",
				LogIndex:    9,
				FromAddress: "0x0000000000000000000000000000000000000bbb",
				ToAddress:   "0x0000000000000000000000000000000000000011",
				ValueRaw:    "1000000",
			},
		},
	}
	resolver := &stubDepositUserResolver{
		usersByAddress: map[string]int64{
			"0x0000000000000000000000000000000000000011": 101,
		},
	}

	watcher := NewUSDCDepositWatcherService(repo, resolver, rpc, USDCDepositWatcherConfig{
		Chain:                  "base",
		USDCContract:           "0x0000000000000000000000000000000000000usd",
		ConfirmationsRequired:  6,
		MaxBlocksPerScanChunk:  50,
		USDCDecimalsMultiplier: 1_000_000,
	})

	err := watcher.ScanOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, repo.creditedCount())
	credit := repo.singleCreditCall(t)
	require.Equal(t, int64(101), credit.userID)
	require.Equal(t, 1.0, credit.amount)
	require.Equal(t, int64(114), repo.scanState("base"))
	require.Len(t, rpc.filters, 1)
	require.Equal(t, EVMTransferLogFilter{
		Chain:       "base",
		Contract:    "0x0000000000000000000000000000000000000usd",
		ToAddresses: []string{"0x0000000000000000000000000000000000000011"},
		FromBlock:   101,
		ToBlock:     114,
	}, rpc.filters[0])
}

func TestWatcher_DoesNotCreditBelowConfirmationThreshold(t *testing.T) {
	repo := newStubOnchainDepositRepo()
	repo.scanStates["base"] = 100

	rpc := &stubEVMRPCClient{
		latest: 120,
		logs: []EVMTransferLog{
			{
				Chain:       "base",
				Contract:    "0x0000000000000000000000000000000000000usd",
				BlockNumber: 115,
				BlockHash:   "0xblock",
				TXHash:      "0xtx3",
				LogIndex:    3,
				FromAddress: "0x0000000000000000000000000000000000000aaa",
				ToAddress:   "0x0000000000000000000000000000000000000011",
				ValueRaw:    "1000000",
			},
		},
	}
	resolver := &stubDepositUserResolver{
		usersByAddress: map[string]int64{
			"0x0000000000000000000000000000000000000011": 101,
		},
	}

	watcher := NewUSDCDepositWatcherService(repo, resolver, rpc, USDCDepositWatcherConfig{
		Chain:                  "base",
		USDCContract:           "0x0000000000000000000000000000000000000usd",
		ConfirmationsRequired:  6,
		MaxBlocksPerScanChunk:  50,
		USDCDecimalsMultiplier: 1_000_000,
	})

	err := watcher.ScanOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 0, repo.creditedCount())
	require.Equal(t, int64(114), repo.scanState("base"))
	require.Len(t, rpc.filters, 1)
	require.Equal(t, []string{"0x0000000000000000000000000000000000000011"}, rpc.filters[0].ToAddresses)
}

func TestWatcher_ReprocessingSameLogDoesNotDoubleCredit(t *testing.T) {
	repo := newStubOnchainDepositRepo()
	repo.scanStates["base"] = 100

	rpc := &stubEVMRPCClient{
		latest: 120,
		logs: []EVMTransferLog{
			{
				Chain:       "base",
				Contract:    "0x0000000000000000000000000000000000000usd",
				BlockNumber: 112,
				BlockHash:   "0xblock",
				TXHash:      "0xtxdup",
				LogIndex:    7,
				FromAddress: "0x0000000000000000000000000000000000000aaa",
				ToAddress:   "0x0000000000000000000000000000000000000011",
				ValueRaw:    "1000000",
			},
			{
				Chain:       "base",
				Contract:    "0x0000000000000000000000000000000000000usd",
				BlockNumber: 112,
				BlockHash:   "0xblock",
				TXHash:      "0xtxdup",
				LogIndex:    7,
				FromAddress: "0x0000000000000000000000000000000000000aaa",
				ToAddress:   "0x0000000000000000000000000000000000000011",
				ValueRaw:    "1000000",
			},
		},
	}
	resolver := &stubDepositUserResolver{
		usersByAddress: map[string]int64{
			"0x0000000000000000000000000000000000000011": 101,
		},
	}

	watcher := NewUSDCDepositWatcherService(repo, resolver, rpc, USDCDepositWatcherConfig{
		Chain:                  "base",
		USDCContract:           "0x0000000000000000000000000000000000000usd",
		ConfirmationsRequired:  6,
		MaxBlocksPerScanChunk:  50,
		USDCDecimalsMultiplier: 1_000_000,
	})

	err := watcher.ScanOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, repo.creditedCount())
	credit := repo.singleCreditCall(t)
	require.Equal(t, int64(101), credit.userID)
	require.Equal(t, 1.0, credit.amount)
}

func TestWatcher_AdvancesCursorOnlyAfterChunkSuccess(t *testing.T) {
	repo := newStubOnchainDepositRepo()
	repo.scanStates["base"] = 100
	repo.upsertErr = errors.New("upsert failed")

	rpc := &stubEVMRPCClient{
		latest: 120,
		logs: []EVMTransferLog{
			{
				Chain:       "base",
				Contract:    "0x0000000000000000000000000000000000000usd",
				BlockNumber: 112,
				BlockHash:   "0xblock",
				TXHash:      "0xtx4",
				LogIndex:    4,
				FromAddress: "0x0000000000000000000000000000000000000aaa",
				ToAddress:   "0x0000000000000000000000000000000000000011",
				ValueRaw:    "1000000",
			},
		},
	}
	resolver := &stubDepositUserResolver{
		usersByAddress: map[string]int64{
			"0x0000000000000000000000000000000000000011": 101,
		},
	}

	watcher := NewUSDCDepositWatcherService(repo, resolver, rpc, USDCDepositWatcherConfig{
		Chain:                  "base",
		USDCContract:           "0x0000000000000000000000000000000000000usd",
		ConfirmationsRequired:  6,
		MaxBlocksPerScanChunk:  50,
		USDCDecimalsMultiplier: 1_000_000,
	})

	err := watcher.ScanOnce(context.Background())
	require.Error(t, err)
	require.Equal(t, int64(100), repo.scanState("base"))

	repo.upsertErr = nil
	err = watcher.ScanOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(114), repo.scanState("base"))
	require.Equal(t, 1, repo.creditedCount())
	credit := repo.singleCreditCall(t)
	require.Equal(t, int64(101), credit.userID)
	require.Equal(t, 1.0, credit.amount)
}

func TestWatcher_SkipsScanRoundWhenNoWatchAddresses(t *testing.T) {
	repo := newStubOnchainDepositRepo()
	repo.scanStates["base"] = 100

	rpc := &stubEVMRPCClient{
		latest: 120,
		logs: []EVMTransferLog{
			{
				Chain:       "base",
				Contract:    "0x0000000000000000000000000000000000000usd",
				BlockNumber: 112,
				BlockHash:   "0xblock",
				TXHash:      "0xtx5",
				LogIndex:    5,
				FromAddress: "0x0000000000000000000000000000000000000aaa",
				ToAddress:   "0x0000000000000000000000000000000000000011",
				ValueRaw:    "1000000",
			},
		},
	}
	resolver := &stubDepositUserResolver{usersByAddress: map[string]int64{}}

	watcher := NewUSDCDepositWatcherService(repo, resolver, rpc, USDCDepositWatcherConfig{
		Chain:                  "base",
		USDCContract:           "0x0000000000000000000000000000000000000usd",
		ConfirmationsRequired:  6,
		MaxBlocksPerScanChunk:  50,
		USDCDecimalsMultiplier: 1_000_000,
	})

	err := watcher.ScanOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(100), repo.scanState("base"))
	require.Equal(t, 0, repo.creditedCount())
	require.Empty(t, rpc.filters)
}

func TestWatcher_HonorsStartBlockOnFirstScanState(t *testing.T) {
	repo := newStubOnchainDepositRepo()

	rpc := &stubEVMRPCClient{
		latest: 120,
		logs: []EVMTransferLog{
			{
				Chain:       "base",
				Contract:    "0x0000000000000000000000000000000000000usd",
				BlockNumber: 109,
				BlockHash:   "0xblock",
				TXHash:      "0xtx-before-start",
				LogIndex:    1,
				FromAddress: "0x0000000000000000000000000000000000000aaa",
				ToAddress:   "0x0000000000000000000000000000000000000011",
				ValueRaw:    "1000000",
			},
			{
				Chain:       "base",
				Contract:    "0x0000000000000000000000000000000000000usd",
				BlockNumber: 110,
				BlockHash:   "0xblock",
				TXHash:      "0xtx-at-start",
				LogIndex:    2,
				FromAddress: "0x0000000000000000000000000000000000000bbb",
				ToAddress:   "0x0000000000000000000000000000000000000011",
				ValueRaw:    "1000000",
			},
		},
	}
	resolver := &stubDepositUserResolver{
		usersByAddress: map[string]int64{
			"0x0000000000000000000000000000000000000011": 101,
		},
	}

	watcher := NewUSDCDepositWatcherService(repo, resolver, rpc, USDCDepositWatcherConfig{
		Chain:                  "base",
		USDCContract:           "0x0000000000000000000000000000000000000usd",
		ConfirmationsRequired:  6,
		MaxBlocksPerScanChunk:  50,
		StartBlock:             110,
		USDCDecimalsMultiplier: 1_000_000,
	})

	err := watcher.ScanOnce(context.Background())
	require.NoError(t, err)
	require.Len(t, rpc.filters, 1)
	require.Equal(t, uint64(110), rpc.filters[0].FromBlock)
	require.Equal(t, uint64(114), rpc.filters[0].ToBlock)
	require.Equal(t, int64(114), repo.scanState("base"))
	require.Equal(t, 1, repo.creditedCount())
}

func TestWatcher_ScansMultipleWatchAddressesInSingleRPCFilterCallPerChunk(t *testing.T) {
	repo := newStubOnchainDepositRepo()
	repo.scanStates["base"] = 100

	rpc := &stubEVMRPCClient{
		latest: 120,
		logs: []EVMTransferLog{
			{
				Chain:       "base",
				Contract:    "0x0000000000000000000000000000000000000usd",
				BlockNumber: 112,
				BlockHash:   "0xblock",
				TXHash:      "0xtx-a1",
				LogIndex:    1,
				FromAddress: "0x0000000000000000000000000000000000000aaa",
				ToAddress:   "0x0000000000000000000000000000000000000011",
				ValueRaw:    "1000000",
			},
			{
				Chain:       "base",
				Contract:    "0x0000000000000000000000000000000000000usd",
				BlockNumber: 113,
				BlockHash:   "0xblock",
				TXHash:      "0xtx-a2",
				LogIndex:    2,
				FromAddress: "0x0000000000000000000000000000000000000bbb",
				ToAddress:   "0x0000000000000000000000000000000000000022",
				ValueRaw:    "2000000",
			},
		},
	}
	resolver := &stubDepositUserResolver{
		usersByAddress: map[string]int64{
			"0x0000000000000000000000000000000000000011": 101,
			"0x0000000000000000000000000000000000000022": 202,
		},
	}

	watcher := NewUSDCDepositWatcherService(repo, resolver, rpc, USDCDepositWatcherConfig{
		Chain:                  "base",
		USDCContract:           "0x0000000000000000000000000000000000000usd",
		ConfirmationsRequired:  6,
		MaxBlocksPerScanChunk:  50,
		USDCDecimalsMultiplier: 1_000_000,
	})

	err := watcher.ScanOnce(context.Background())
	require.NoError(t, err)
	require.Len(t, rpc.filters, 1)
	require.Equal(t, uint64(101), rpc.filters[0].FromBlock)
	require.Equal(t, uint64(114), rpc.filters[0].ToBlock)
	require.ElementsMatch(t, []string{
		"0x0000000000000000000000000000000000000011",
		"0x0000000000000000000000000000000000000022",
	}, rpc.filters[0].ToAddresses)
	require.Equal(t, 2, repo.creditedCount())
	require.Equal(t, int64(114), repo.scanState("base"))
}

func TestUSDCDepositWatcherService_StopIsSafe(t *testing.T) {
	svc := NewUSDCDepositWatcherService(nil, nil, nil, USDCDepositWatcherConfig{})
	require.NotPanics(t, func() { svc.Stop() })
}
