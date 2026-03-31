package service

import "context"

type EVMRPCClient interface {
	LatestBlock(ctx context.Context, chain string) (uint64, error)
	GetERC20TransferLogs(ctx context.Context, filter EVMTransferLogFilter) ([]EVMTransferLog, error)
}
