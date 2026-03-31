//go:build unit

package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestBuildERC20TransferFilterQuery_WithToAddressTopic(t *testing.T) {
	filter := service.EVMTransferLogFilter{
		Chain:     "base",
		Contract:  "0x1111111111111111111111111111111111111111",
		ToAddress: "0x2222222222222222222222222222222222222222",
		FromBlock: 100,
		ToBlock:   200,
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
	require.Equal(t, []common.Hash{topicAddressHash(filter.ToAddress)}, query.Topics[2])
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
