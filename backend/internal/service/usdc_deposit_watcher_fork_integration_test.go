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

			logs, err := rpcClient.GetERC20TransferLogs(ctx, service.EVMTransferLogFilter{
				Chain:       chain.name,
				Contract:    chain.usdcContract,
				ToAddresses: []string{"0x0000000000000000000000000000000000000001"},
				FromBlock:   latest,
				ToBlock:     latest,
			})
			require.NoError(t, err)
			require.NotNil(t, logs)
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
