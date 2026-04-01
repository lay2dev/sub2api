//go:build integration

package service_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUSDCDepositWatcherForkRPC_Compatibility(t *testing.T) {
	t.Parallel()

	chains := []struct {
		name          string
		envVar        string
		usdcContract  string
		confirmations uint64
	}{
		{
			name:          "bsc",
			envVar:        "BSC_FORK_RPC_URL",
			usdcContract:  "0x8ac76a51cc950d9822d68b83fe1ad97b32cd580d",
			confirmations: 15,
		},
		{
			name:          "arbitrum",
			envVar:        "ARBITRUM_FORK_RPC_URL",
			usdcContract:  "0xaf88d065e77c8cc2239327c5edb3a432268e5831",
			confirmations: 20,
		},
		{
			name:          "base",
			envVar:        "BASE_FORK_RPC_URL",
			usdcContract:  "0x833589fcd6edb6e08f4c7c32d4f71b54bda02913",
			confirmations: 12,
		},
	}

	for _, chain := range chains {
		chain := chain
		t.Run(chain.name, func(t *testing.T) {
			t.Parallel()

			rpcURL := strings.TrimSpace(os.Getenv(chain.envVar))
			if rpcURL == "" {
				t.Skipf("skipping %s fork RPC compatibility test: %s is not set", chain.name, chain.envVar)
			}

			rpcClient := repository.NewEVMRPCClient(buildForkRPCConfig(chain.name, rpcURL, chain.usdcContract, chain.confirmations))
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			latest, err := rpcClient.LatestBlock(ctx, chain.name)
			require.NoError(t, err)
			require.Greater(t, latest, uint64(0))

			logs, err := rpcClient.GetERC20TransferLogs(ctx, service.EVMTransferLogFilter{
				Chain:       chain.name,
				Contract:    chain.usdcContract,
				ToAddresses: []string{"0x0000000000000000000000000000000000000001"},
				FromBlock:   latest,
				ToBlock:     latest,
			})
			require.NoError(t, err)
			require.NotNil(t, logs)

			repo := &forkWatcherRepo{
				scanStates: map[string]int64{
					chain.name: int64(latest - 1),
				},
			}
			resolver := &forkWatcherResolver{
				userIDByAddress: map[string]int64{
					"0x0000000000000000000000000000000000000001": 1,
				},
			}
			watcher := service.NewUSDCDepositWatcherService(repo, resolver, rpcClient, service.USDCDepositWatcherConfig{
				Chain:                  chain.name,
				USDCContract:           chain.usdcContract,
				ConfirmationsRequired:  0,
				StartBlock:             latest,
				MaxBlocksPerScanChunk:  1,
				USDCDecimalsMultiplier: 1_000_000,
			})
			require.NoError(t, watcher.ScanOnce(ctx))
			require.Equal(t, int64(latest), repo.scanStates[chain.name])
		})
	}
}

func buildForkRPCConfig(chain, rpcURL, usdcContract string, confirmations uint64) *config.Config {
	cfg := &config.Config{
		Wallet: config.WalletConfig{
			Deposits: config.WalletDepositsConfig{
				Enabled:               true,
				PollIntervalSeconds:   30,
				ScanChunkSize:         1,
				RequestTimeoutSeconds: 20,
			},
		},
	}

	switch chain {
	case "bsc":
		cfg.Wallet.Deposits.Chains.BSC = config.WalletDepositChainConfig{
			Enabled:             true,
			RPCURL:              rpcURL,
			USDCContractAddress: usdcContract,
			Confirmations:       confirmations,
		}
	case "arbitrum":
		cfg.Wallet.Deposits.Chains.Arbitrum = config.WalletDepositChainConfig{
			Enabled:             true,
			RPCURL:              rpcURL,
			USDCContractAddress: usdcContract,
			Confirmations:       confirmations,
		}
	case "base":
		cfg.Wallet.Deposits.Chains.Base = config.WalletDepositChainConfig{
			Enabled:             true,
			RPCURL:              rpcURL,
			USDCContractAddress: usdcContract,
			Confirmations:       confirmations,
		}
	}

	return cfg
}

type forkWatcherResolver struct {
	userIDByAddress map[string]int64
}

func (r *forkWatcherResolver) ResolveUserIDByAddress(_ context.Context, address string) (int64, bool, error) {
	userID, ok := r.userIDByAddress[strings.ToLower(address)]
	return userID, ok, nil
}

func (r *forkWatcherResolver) ListBindingAddresses(_ context.Context) ([]string, error) {
	out := make([]string, 0, len(r.userIDByAddress))
	for address := range r.userIDByAddress {
		out = append(out, address)
	}
	return out, nil
}

type forkWatcherRepo struct {
	scanStates map[string]int64
}

func (r *forkWatcherRepo) GetByID(_ context.Context, _ int64) (*service.OnchainDeposit, error) {
	return nil, nil
}

func (r *forkWatcherRepo) GetScanState(_ context.Context, chain string) (*service.OnchainDepositScanState, error) {
	return &service.OnchainDepositScanState{
		Chain:            chain,
		LastScannedBlock: r.scanStates[chain],
	}, nil
}

func (r *forkWatcherRepo) UpsertScanState(_ context.Context, chain string, lastScannedBlock int64) error {
	r.scanStates[chain] = lastScannedBlock
	return nil
}

func (r *forkWatcherRepo) CreateOrGetDetected(_ context.Context, deposit *service.OnchainDeposit) (*service.OnchainDeposit, error) {
	cloned := *deposit
	cloned.ID = 1
	return &cloned, nil
}

func (r *forkWatcherRepo) CreditDepositAndBalance(_ context.Context, _ int64, _ int64, _ float64) error {
	return nil
}

func (r *forkWatcherRepo) ListPendingFailed(_ context.Context, _ string, _ int) ([]service.OnchainDeposit, error) {
	return nil, nil
}
