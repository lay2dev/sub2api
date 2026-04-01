package repository

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

var erc20TransferTopic = crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))

type ethLogReader interface {
	BlockNumber(ctx context.Context) (uint64, error)
	FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error)
}

type ethClientDialer func(ctx context.Context, endpoint string) (ethLogReader, error)

type evmRPCClient struct {
	mu          sync.RWMutex
	endpoints   map[string]string
	clients     map[string]ethLogReader
	chainDialMu map[string]*sync.Mutex
	dial        ethClientDialer
}

func NewEVMRPCClient(cfg *config.Config) service.EVMRPCClient {
	return &evmRPCClient{
		endpoints:   buildEVMRPCEndpoints(cfg),
		clients:     map[string]ethLogReader{},
		chainDialMu: map[string]*sync.Mutex{},
		dial:        defaultEthClientDial,
	}
}

func buildEVMRPCEndpoints(cfg *config.Config) map[string]string {
	if cfg == nil {
		return map[string]string{}
	}

	endpoints := make(map[string]string, 3)
	chains := []struct {
		name string
		cfg  config.WalletDepositChainConfig
	}{
		{name: "bsc", cfg: cfg.Wallet.Deposits.Chains.BSC},
		{name: "arbitrum", cfg: cfg.Wallet.Deposits.Chains.Arbitrum},
		{name: "base", cfg: cfg.Wallet.Deposits.Chains.Base},
	}
	for _, chain := range chains {
		if !chain.cfg.Enabled {
			continue
		}
		endpoint := strings.TrimSpace(chain.cfg.RPCURL)
		if endpoint == "" {
			continue
		}
		endpoints[chain.name] = endpoint
	}
	return endpoints
}

func defaultEthClientDial(ctx context.Context, endpoint string) (ethLogReader, error) {
	return ethclient.DialContext(ctx, endpoint)
}

func (c *evmRPCClient) LatestBlock(ctx context.Context, chain string) (uint64, error) {
	client, err := c.clientForChain(ctx, chain)
	if err != nil {
		return 0, err
	}
	return client.BlockNumber(ctx)
}

func (c *evmRPCClient) GetERC20TransferLogs(ctx context.Context, filter service.EVMTransferLogFilter) ([]service.EVMTransferLog, error) {
	client, err := c.clientForChain(ctx, filter.Chain)
	if err != nil {
		return nil, err
	}
	query, err := buildERC20TransferFilterQuery(filter)
	if err != nil {
		return nil, err
	}

	rawLogs, err := client.FilterLogs(ctx, query)
	if err != nil {
		return nil, err
	}
	out := make([]service.EVMTransferLog, 0, len(rawLogs))
	for _, rawLog := range rawLogs {
		out = append(out, evmLogToTransferLog(filter.Chain, rawLog))
	}
	return out, nil
}

func (c *evmRPCClient) clientForChain(ctx context.Context, chain string) (ethLogReader, error) {
	chain = strings.ToLower(strings.TrimSpace(chain))
	if chain == "" {
		return nil, fmt.Errorf("chain is required")
	}

	c.mu.RLock()
	if client, ok := c.clients[chain]; ok {
		c.mu.RUnlock()
		return client, nil
	}
	endpoint := c.endpoints[chain]
	c.mu.RUnlock()

	if endpoint == "" {
		return nil, fmt.Errorf("rpc endpoint is not configured for chain=%s", chain)
	}

	lock := c.getChainDialLock(chain)
	lock.Lock()
	defer lock.Unlock()

	c.mu.RLock()
	if client, ok := c.clients[chain]; ok {
		c.mu.RUnlock()
		return client, nil
	}
	c.mu.RUnlock()

	client, err := c.dial(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("dial chain=%s rpc: %w", chain, err)
	}

	c.mu.Lock()
	c.clients[chain] = client
	c.mu.Unlock()
	return client, nil
}

func buildERC20TransferFilterQuery(filter service.EVMTransferLogFilter) (ethereum.FilterQuery, error) {
	if filter.FromBlock > filter.ToBlock {
		return ethereum.FilterQuery{}, fmt.Errorf("invalid block range: from=%d to=%d", filter.FromBlock, filter.ToBlock)
	}

	contract := strings.TrimSpace(filter.Contract)
	if !common.IsHexAddress(contract) {
		return ethereum.FilterQuery{}, fmt.Errorf("invalid contract address: %q", filter.Contract)
	}

	topics := [][]common.Hash{{erc20TransferTopic}}
	toTopicHashes, err := buildToAddressTopicHashes(filter.ToAddresses)
	if err != nil {
		return ethereum.FilterQuery{}, err
	}
	if len(toTopicHashes) > 0 {
		topics = append(topics, nil, toTopicHashes)
	}

	return ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(filter.FromBlock),
		ToBlock:   new(big.Int).SetUint64(filter.ToBlock),
		Addresses: []common.Address{common.HexToAddress(contract)},
		Topics:    topics,
	}, nil
}

func (c *evmRPCClient) getChainDialLock(chain string) *sync.Mutex {
	c.mu.Lock()
	defer c.mu.Unlock()
	lock, ok := c.chainDialMu[chain]
	if ok {
		return lock
	}
	lock = &sync.Mutex{}
	c.chainDialMu[chain] = lock
	return lock
}

func buildToAddressTopicHashes(addresses []string) ([]common.Hash, error) {
	if len(addresses) == 0 {
		return nil, nil
	}

	unique := make(map[string]struct{}, len(addresses))
	out := make([]common.Hash, 0, len(addresses))
	for _, address := range addresses {
		toAddress := strings.TrimSpace(address)
		if toAddress == "" {
			continue
		}
		if !common.IsHexAddress(toAddress) {
			return nil, fmt.Errorf("invalid to_address: %q", address)
		}

		normalized := strings.ToLower(toAddress)
		if _, ok := unique[normalized]; ok {
			continue
		}
		unique[normalized] = struct{}{}
		out = append(out, topicAddressHash(toAddress))
	}
	return out, nil
}

func topicAddressHash(address string) common.Hash {
	return common.BytesToHash(common.HexToAddress(strings.TrimSpace(address)).Bytes())
}

func evmLogToTransferLog(chain string, raw types.Log) service.EVMTransferLog {
	out := service.EVMTransferLog{
		Chain:       strings.ToLower(strings.TrimSpace(chain)),
		Contract:    strings.ToLower(raw.Address.Hex()),
		BlockNumber: raw.BlockNumber,
		BlockHash:   strings.ToLower(raw.BlockHash.Hex()),
		TXHash:      strings.ToLower(raw.TxHash.Hex()),
		LogIndex:    uint64(raw.Index),
		ValueRaw:    parseTransferValueRaw(raw.Data),
	}
	if len(raw.Topics) > 1 {
		out.FromAddress = strings.ToLower(topicHashToAddress(raw.Topics[1]).Hex())
	}
	if len(raw.Topics) > 2 {
		out.ToAddress = strings.ToLower(topicHashToAddress(raw.Topics[2]).Hex())
	}
	return out
}

func topicHashToAddress(topic common.Hash) common.Address {
	topicBytes := topic.Bytes()
	if len(topicBytes) < common.AddressLength {
		return common.Address{}
	}
	return common.BytesToAddress(topicBytes[len(topicBytes)-common.AddressLength:])
}

func parseTransferValueRaw(data []byte) string {
	if len(data) == 0 {
		return "0"
	}
	return new(big.Int).SetBytes(data).String()
}
