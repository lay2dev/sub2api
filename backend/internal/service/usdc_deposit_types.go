package service

type EVMTransferLog struct {
	Chain       string
	Contract    string
	BlockNumber uint64
	BlockHash   string
	TXHash      string
	LogIndex    uint64
	FromAddress string
	ToAddress   string
	ValueRaw    string
}

type EVMTransferLogFilter struct {
	Chain       string
	Contract    string
	ToAddresses []string
	FromBlock   uint64
	ToBlock     uint64
}
