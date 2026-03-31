package service

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/big"
	"sort"
	"strings"
	"sync"
	"time"
)

const defaultUSDCDecimalsMultiplier int64 = 1_000_000

type DepositUserResolver interface {
	ResolveUserIDByAddress(ctx context.Context, address string) (int64, bool, error)
	ListBindingAddresses(ctx context.Context) ([]string, error)
}

type USDCDepositWatcherConfig struct {
	Chain                  string
	USDCContract           string
	ConfirmationsRequired  uint64
	StartBlock             uint64
	MaxBlocksPerScanChunk  uint64
	USDCDecimalsMultiplier int64
	PollInterval           time.Duration
	RequestTimeout         time.Duration
}

type USDCDepositWatcherService struct {
	depositRepo  OnchainDepositRepository
	userResolver DepositUserResolver
	rpcClient    EVMRPCClient
	cfg          USDCDepositWatcherConfig
	children     []*USDCDepositWatcherService
	stopCh       chan struct{}
	stopOnce     sync.Once
	startOnce    sync.Once
	wg           sync.WaitGroup
}

func NewUSDCDepositWatcherService(
	depositRepo OnchainDepositRepository,
	userResolver DepositUserResolver,
	rpcClient EVMRPCClient,
	cfg USDCDepositWatcherConfig,
) *USDCDepositWatcherService {
	if cfg.MaxBlocksPerScanChunk == 0 {
		cfg.MaxBlocksPerScanChunk = 1
	}
	if cfg.USDCDecimalsMultiplier <= 0 {
		cfg.USDCDecimalsMultiplier = defaultUSDCDecimalsMultiplier
	}
	cfg.Chain = strings.ToLower(strings.TrimSpace(cfg.Chain))
	cfg.USDCContract = normalizeAddress(cfg.USDCContract)

	return &USDCDepositWatcherService{
		depositRepo:  depositRepo,
		userResolver: userResolver,
		rpcClient:    rpcClient,
		cfg:          cfg,
		stopCh:       make(chan struct{}),
	}
}

func NewUSDCDepositWatcherServiceGroup(children []*USDCDepositWatcherService) *USDCDepositWatcherService {
	out := make([]*USDCDepositWatcherService, 0, len(children))
	for _, child := range children {
		if child == nil {
			continue
		}
		out = append(out, child)
	}
	return &USDCDepositWatcherService{children: out}
}

func (s *USDCDepositWatcherService) Start() {
	if s == nil {
		return
	}
	if len(s.children) > 0 {
		s.startOnce.Do(func() {
			for _, child := range s.children {
				child.Start()
			}
		})
		return
	}
	if s.depositRepo == nil || s.userResolver == nil || s.rpcClient == nil {
		return
	}
	if s.cfg.Chain == "" || s.cfg.USDCContract == "" || s.cfg.PollInterval <= 0 {
		return
	}

	s.startOnce.Do(func() {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.scanRound()

			ticker := time.NewTicker(s.cfg.PollInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					s.scanRound()
				case <-s.stopCh:
					return
				}
			}
		}()
	})
}

func (s *USDCDepositWatcherService) Stop() {
	if s == nil {
		return
	}
	if len(s.children) > 0 {
		s.stopOnce.Do(func() {
			for _, child := range s.children {
				child.Stop()
			}
		})
		return
	}
	s.stopOnce.Do(func() {
		if s.stopCh != nil {
			close(s.stopCh)
		}
	})
	s.wg.Wait()
}

func (s *USDCDepositWatcherService) scanRound() {
	ctx := context.Background()
	cancel := func() {}
	if s.cfg.RequestTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, s.cfg.RequestTimeout)
	}
	defer cancel()

	if err := s.ScanOnce(ctx); err != nil {
		log.Printf("[USDCDepositWatcher] scan failed for chain=%s: %v", s.cfg.Chain, err)
	}
}

func (s *USDCDepositWatcherService) ScanOnce(ctx context.Context) error {
	if s == nil {
		return fmt.Errorf("watcher is not initialized")
	}
	if len(s.children) > 0 {
		for _, child := range s.children {
			if err := child.ScanOnce(ctx); err != nil {
				return err
			}
		}
		return nil
	}
	if s == nil || s.depositRepo == nil || s.userResolver == nil || s.rpcClient == nil {
		return fmt.Errorf("watcher dependencies are not initialized")
	}

	latestBlock, err := s.rpcClient.LatestBlock(ctx, s.cfg.Chain)
	if err != nil {
		return err
	}
	if latestBlock < s.cfg.ConfirmationsRequired {
		return nil
	}

	scanState, err := s.depositRepo.GetScanState(ctx, s.cfg.Chain)
	if err != nil {
		return err
	}

	var cursor uint64
	if scanState != nil && scanState.LastScannedBlock > 0 {
		cursor = uint64(scanState.LastScannedBlock)
	} else if s.cfg.StartBlock > 0 {
		cursor = s.cfg.StartBlock - 1
	}

	safeBlock := latestBlock - s.cfg.ConfirmationsRequired
	if cursor >= safeBlock {
		return nil
	}

	watchAddresses, err := s.resolveWatchAddresses(ctx)
	if err != nil {
		return err
	}
	if len(watchAddresses) == 0 {
		return nil
	}

	for from := cursor + 1; from <= safeBlock; {
		to := from + s.cfg.MaxBlocksPerScanChunk - 1
		if to > safeBlock {
			to = safeBlock
		}

		if err := s.scanChunk(ctx, from, to, watchAddresses); err != nil {
			return err
		}
		if err := s.depositRepo.UpsertScanState(ctx, s.cfg.Chain, int64(to)); err != nil {
			return err
		}

		if to == math.MaxUint64 {
			break
		}
		from = to + 1
	}

	return nil
}

func (s *USDCDepositWatcherService) scanChunk(ctx context.Context, from, to uint64, watchAddresses []string) error {
	if len(watchAddresses) == 0 {
		return nil
	}

	toAddressFilter := watchAddresses[0]
	if len(watchAddresses) > 1 {
		toAddressFilter = strings.Join(watchAddresses, ",")
	}

	logs, err := s.rpcClient.GetERC20TransferLogs(ctx, EVMTransferLogFilter{
		Chain:     s.cfg.Chain,
		Contract:  s.cfg.USDCContract,
		ToAddress: toAddressFilter,
		FromBlock: from,
		ToBlock:   to,
	})
	if err != nil {
		return err
	}
	if err := s.processLogs(ctx, logs); err != nil {
		return err
	}
	return nil
}

func (s *USDCDepositWatcherService) processLogs(ctx context.Context, logs []EVMTransferLog) error {
	for _, log := range logs {
		userID, ok, err := s.userResolver.ResolveUserIDByAddress(ctx, log.ToAddress)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if err := s.processLog(ctx, userID, log); err != nil {
			return err
		}
	}
	return nil
}

func (s *USDCDepositWatcherService) processLog(ctx context.Context, userID int64, log EVMTransferLog) error {
	amount, err := parseUSDCAmountWithMultiplier(log.ValueRaw, s.cfg.USDCDecimalsMultiplier)
	if err != nil {
		return err
	}

	logIndex, err := uint64ToInt64(log.LogIndex)
	if err != nil {
		return err
	}
	blockNumber, err := uint64ToInt64(log.BlockNumber)
	if err != nil {
		return err
	}

	deposit, err := s.depositRepo.CreateOrGetDetected(ctx, &OnchainDeposit{
		UserID:        userID,
		Chain:         s.cfg.Chain,
		TokenSymbol:   "USDC",
		TokenContract: s.cfg.USDCContract,
		TXHash:        strings.ToLower(strings.TrimSpace(log.TXHash)),
		LogIndex:      logIndex,
		BlockNumber:   blockNumber,
		BlockHash:     strings.ToLower(strings.TrimSpace(log.BlockHash)),
		FromAddress:   normalizeAddress(log.FromAddress),
		ToAddress:     normalizeAddress(log.ToAddress),
		AmountRaw:     strings.TrimSpace(log.ValueRaw),
		AmountCredit:  amount,
		Status:        OnchainDepositStatusDetected,
	})
	if err != nil {
		return err
	}
	if deposit == nil || deposit.Status == OnchainDepositStatusCredited {
		return nil
	}

	creditUserID := deposit.UserID
	if creditUserID == 0 {
		creditUserID = userID
	}
	return s.depositRepo.CreditDepositAndBalance(ctx, deposit.ID, creditUserID, amount)
}

func parseUSDCAmount(raw string) (float64, error) {
	return parseUSDCAmountWithMultiplier(raw, defaultUSDCDecimalsMultiplier)
}

func parseUSDCAmountWithMultiplier(raw string, multiplier int64) (float64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, fmt.Errorf("empty amount")
	}
	if multiplier <= 0 {
		return 0, fmt.Errorf("invalid decimals multiplier: %d", multiplier)
	}

	amountInt := new(big.Int)
	if _, ok := amountInt.SetString(value, 10); !ok {
		return 0, fmt.Errorf("invalid amount: %q", raw)
	}
	if amountInt.Sign() < 0 {
		return 0, fmt.Errorf("amount must be non-negative")
	}

	amountRat := new(big.Rat).SetInt(amountInt)
	amountRat.Quo(amountRat, big.NewRat(multiplier, 1))
	amountFloat, _ := amountRat.Float64()
	return amountFloat, nil
}

func uint64ToInt64(v uint64) (int64, error) {
	if v > math.MaxInt64 {
		return 0, fmt.Errorf("value overflows int64: %d", v)
	}
	return int64(v), nil
}

func normalizeAddress(address string) string {
	return strings.ToLower(strings.TrimSpace(address))
}

func (s *USDCDepositWatcherService) resolveWatchAddresses(ctx context.Context) ([]string, error) {
	addrs, err := s.userResolver.ListBindingAddresses(ctx)
	if err != nil {
		return nil, err
	}
	return normalizeAddresses(addrs), nil
}

func normalizeAddresses(addresses []string) []string {
	if len(addresses) == 0 {
		return nil
	}
	unique := make(map[string]struct{}, len(addresses))
	normalized := make([]string, 0, len(addresses))
	for _, addr := range addresses {
		norm := normalizeAddress(addr)
		if norm == "" {
			continue
		}
		if _, ok := unique[norm]; ok {
			continue
		}
		unique[norm] = struct{}{}
		normalized = append(normalized, norm)
	}
	sort.Strings(normalized)
	return normalized
}
