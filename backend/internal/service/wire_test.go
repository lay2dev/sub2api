package service

import (
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/zeromicro/go-zero/core/collection"
)

func TestProvideTimingWheelService_ReturnsError(t *testing.T) {
	original := newTimingWheel
	t.Cleanup(func() { newTimingWheel = original })

	newTimingWheel = func(_ time.Duration, _ int, _ collection.Execute) (*collection.TimingWheel, error) {
		return nil, errors.New("boom")
	}

	svc, err := ProvideTimingWheelService()
	if err == nil {
		t.Fatalf("期望返回 error，但得到 nil")
	}
	if svc != nil {
		t.Fatalf("期望返回 nil svc，但得到非空")
	}
}

func TestProvideTimingWheelService_Success(t *testing.T) {
	svc, err := ProvideTimingWheelService()
	if err != nil {
		t.Fatalf("期望 err 为 nil，但得到: %v", err)
	}
	if svc == nil {
		t.Fatalf("期望 svc 非空，但得到 nil")
	}
	svc.Stop()
}

func TestWatcher_ProvideUSDCDepositWatcherServiceBuildsMultiChainChildren(t *testing.T) {
	cfg := &config.Config{
		Wallet: config.WalletConfig{
			Deposits: config.WalletDepositsConfig{
				Enabled:               true,
				PollIntervalSeconds:   30,
				ScanChunkSize:         500,
				RequestTimeoutSeconds: 15,
				Chains: config.WalletDepositChainsConfig{
					BSC: config.WalletDepositChainConfig{
						Enabled:             true,
						USDCContractAddress: "0x1111111111111111111111111111111111111111",
						Confirmations:       15,
						StartBlock:          101,
					},
					Arbitrum: config.WalletDepositChainConfig{
						Enabled:             true,
						USDCContractAddress: "0x2222222222222222222222222222222222222222",
						Confirmations:       20,
						StartBlock:          202,
					},
					Base: config.WalletDepositChainConfig{
						Enabled:             true,
						USDCContractAddress: "0x3333333333333333333333333333333333333333",
						Confirmations:       25,
						StartBlock:          303,
					},
				},
			},
		},
	}

	watcher := ProvideUSDCDepositWatcherService(nil, nil, nil, cfg)
	if watcher == nil {
		t.Fatalf("expected watcher to be non-nil")
	}
	if len(watcher.children) != 3 {
		t.Fatalf("expected 3 child watchers, got %d", len(watcher.children))
	}

	expectedStartBlocks := map[string]uint64{
		"bsc":      101,
		"arbitrum": 202,
		"base":     303,
	}
	for _, child := range watcher.children {
		if child == nil {
			t.Fatalf("expected child watcher to be non-nil")
		}
		want, ok := expectedStartBlocks[child.cfg.Chain]
		if !ok {
			t.Fatalf("unexpected child chain: %s", child.cfg.Chain)
		}
		if child.cfg.StartBlock != want {
			t.Fatalf("expected start_block=%d for chain=%s, got %d", want, child.cfg.Chain, child.cfg.StartBlock)
		}
	}

	// Group stop should be safe and idempotent even with child watchers.
	watcher.Stop()
	watcher.Stop()
}
