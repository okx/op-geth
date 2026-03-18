package monitor

// TransactionProcessId represents different stages in the transaction lifecycle
// These IDs match the reth implementation for consistency
type TransactionProcessId uint64

const (
	// RPC node: Transaction received and ready to forward
	RpcReceiveTxEnd TransactionProcessId = 15010

	// Sequencer node: Transaction received and added to pool
	SeqReceiveTxEnd TransactionProcessId = 15030

	// Sequencer node: Block building started
	SeqBlockBuildStart TransactionProcessId = 15032

	// Sequencer node: Transaction execution completed
	SeqTxExecutionEnd TransactionProcessId = 15034

	// Sequencer node: Block building completed
	SeqBlockBuildEnd TransactionProcessId = 15036

	// Sequencer node: Block sending started
	SeqBlockSendStart TransactionProcessId = 15042

	// RPC node: Block received from sequencer
	RpcBlockReceiveEnd TransactionProcessId = 15060

	// RPC node: Block insertion completed
	RpcBlockInsertEnd TransactionProcessId = 15062
)

// String returns the string representation of the process ID
func (p TransactionProcessId) String() string {
	switch p {
	case RpcReceiveTxEnd:
		return "xlayer_rpc_receive_tx"
	case SeqReceiveTxEnd:
		return "xlayer_seq_receive_tx"
	case SeqBlockBuildStart:
		return "xlayer_seq_begin_block"
	case SeqTxExecutionEnd:
		return "xlayer_seq_package_tx"
	case SeqBlockBuildEnd:
		return "xlayer_seq_end_block"
	case SeqBlockSendStart:
		return "xlayer_seq_ds_sent"
	case RpcBlockReceiveEnd:
		return "xlayer_rpc_receive_block"
	case RpcBlockInsertEnd:
		return "xlayer_rpc_finish_block"
	default:
		return "unknown"
	}
}

// ServiceName returns the service name based on the process ID
func (p TransactionProcessId) ServiceName() string {
	switch p {
	case RpcReceiveTxEnd, RpcBlockReceiveEnd, RpcBlockInsertEnd:
		return RPC_SERVICE_NAME
	case SeqReceiveTxEnd, SeqBlockBuildStart, SeqTxExecutionEnd, SeqBlockBuildEnd, SeqBlockSendStart:
		return SEQ_SERVICE_NAME
	default:
		return "unknown"
	}
}

const (
	// Fixed chain name
	CHAIN_NAME = "X Layer"

	// Fixed business name
	BUSINESS_NAME = "X Layer"

	// Fixed chain ID
	CHAIN_ID = "196"

	// RPC service name
	RPC_SERVICE_NAME = "okx-defi-xlayer-rpcpay-pro"

	// Sequencer service name
	SEQ_SERVICE_NAME = "okx-defi-xlayer-egseqz-pro"
)
