//go:build unit

package repository

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

type stubEthLogReader struct{}

func (s *stubEthLogReader) BlockNumber(_ context.Context) (uint64, error) {
	return 0, nil
}

func (s *stubEthLogReader) FilterLogs(_ context.Context, _ ethereum.FilterQuery) ([]types.Log, error) {
	return nil, nil
}

func TestBuildERC20TransferFilterQuery_WithToAddressTopic(t *testing.T) {
	filter := service.EVMTransferLogFilter{
		Chain:       "base",
		Contract:    "0x1111111111111111111111111111111111111111",
		ToAddresses: []string{"0x2222222222222222222222222222222222222222"},
		FromBlock:   100,
		ToBlock:     200,
	}

	query, err := buildERC20TransferFilterQuery(filter)
	require.NoError(t, err)
	require.Len(t, query.Addresses, 1)
	require.Equal(t, common.HexToAddress(filter.Contract), query.Addresses[0])
	require.Equal(t, filter.FromBlock, query.FromBlock.Uint64())
	require.Equal(t, filter.ToBlock, query.ToBlock.Uint64())

	require.Len(t, query.Topics, 3)
	require.Equal(t, []common.Hash{erc20TransferTopic}, query.Topics[0])
	require.Nil(t, query.Topics[1])
	require.Equal(t, []common.Hash{topicAddressHash(filter.ToAddresses[0])}, query.Topics[2])
}

func TestBuildERC20TransferFilterQuery_WithoutToAddressTopic(t *testing.T) {
	filter := service.EVMTransferLogFilter{
		Chain:     "base",
		Contract:  "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		FromBlock: 1,
		ToBlock:   2,
	}

	query, err := buildERC20TransferFilterQuery(filter)
	require.NoError(t, err)
	require.Len(t, query.Topics, 1)
	require.Equal(t, []common.Hash{erc20TransferTopic}, query.Topics[0])
}

func TestBuildERC20TransferFilterQuery_WithMultipleToAddressesTopic(t *testing.T) {
	filter := service.EVMTransferLogFilter{
		Chain:    "base",
		Contract: "0x1111111111111111111111111111111111111111",
		ToAddresses: []string{
			"0x2222222222222222222222222222222222222222",
			"0x3333333333333333333333333333333333333333",
		},
		FromBlock: 100,
		ToBlock:   200,
	}

	query, err := buildERC20TransferFilterQuery(filter)
	require.NoError(t, err)
	require.Len(t, query.Topics, 3)
	require.Equal(t, []common.Hash{erc20TransferTopic}, query.Topics[0])
	require.Nil(t, query.Topics[1])
	require.Equal(t, []common.Hash{
		topicAddressHash("0x2222222222222222222222222222222222222222"),
		topicAddressHash("0x3333333333333333333333333333333333333333"),
	}, query.Topics[2])
}

func TestEVMRPCClient_ClientForChain_DialDifferentChainsDoesNotBlock(t *testing.T) {
	slowDialStarted := make(chan struct{})
	releaseSlowDial := make(chan struct{})

	var fastDialCalls int32
	client := &evmRPCClient{
		endpoints: map[string]string{
			"bsc":  "slow",
			"base": "fast",
		},
		clients:     map[string]ethLogReader{},
		chainDialMu: map[string]*sync.Mutex{},
		dial: func(_ context.Context, endpoint string) (ethLogReader, error) {
			switch endpoint {
			case "slow":
				close(slowDialStarted)
				<-releaseSlowDial
				return &stubEthLogReader{}, nil
			case "fast":
				atomic.AddInt32(&fastDialCalls, 1)
				return &stubEthLogReader{}, nil
			default:
				return nil, errors.New("unexpected endpoint")
			}
		},
	}

	slowErrCh := make(chan error, 1)
	go func() {
		_, err := client.clientForChain(context.Background(), "bsc")
		slowErrCh <- err
	}()

	select {
	case <-slowDialStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("slow dial did not start")
	}

	fastDone := make(chan error, 1)
	go func() {
		_, err := client.clientForChain(context.Background(), "base")
		fastDone <- err
	}()

	select {
	case err := <-fastDone:
		require.NoError(t, err)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("dial for another chain should not block on slow chain dial")
	}

	close(releaseSlowDial)
	require.NoError(t, <-slowErrCh)
	require.Equal(t, int32(1), atomic.LoadInt32(&fastDialCalls))
}
