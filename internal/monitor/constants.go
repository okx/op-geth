package monitor

// ProcessStep represents a step in transaction processing
type ProcessStep struct {
	ID  uint64
	Key string
}

var (
	// RPC Service Steps
	StepRPCReceiveTx    = ProcessStep{10010, "op_geth_rpc_receive_tx"}
	StepRPCSendTx       = ProcessStep{10012, "op_geth_rpc_send_tx"}
	StepRPCReceiveBlock = ProcessStep{10060, "op_geth_rpc_receive_block"}
	StepRPCFinishBlock  = ProcessStep{10062, "op_geth_rpc_finish_block"}

	// Transaction Pool Steps
	StepTxPoolAdd      = ProcessStep{10020, "op_geth_txpool_add"}
	StepTxPoolValidate = ProcessStep{10022, "op_geth_txpool_validate"}
	StepTxPoolAccept   = ProcessStep{10024, "op_geth_txpool_accept"}
	StepTxPoolReject   = ProcessStep{10026, "op_geth_txpool_reject"}

	// Mining Steps
	StepMinerSelectTx  = ProcessStep{10030, "op_geth_miner_select_tx"}
	StepMinerExecuteTx = ProcessStep{10032, "op_geth_miner_execute_tx"}
	StepMinerPackageTx = ProcessStep{10034, "op_geth_miner_package_tx"}
	StepMinerEndBlock  = ProcessStep{10036, "op_geth_miner_end_block"}

	// State Processing Steps
	StepStateProcessTx       = ProcessStep{10040, "op_geth_state_process_tx"}
	StepStateApplyTx         = ProcessStep{10042, "op_geth_state_apply_tx"}
	StepStateGenerateReceipt = ProcessStep{10044, "op_geth_state_generate_receipt"}
	StepStateCommit          = ProcessStep{10046, "op_geth_state_commit"}

	// Blockchain Steps
	StepBlockchainInsert   = ProcessStep{10050, "op_geth_blockchain_insert"}
	StepBlockchainValidate = ProcessStep{10052, "op_geth_blockchain_validate"}
	StepBlockchainFinalize = ProcessStep{10054, "op_geth_blockchain_finalize"}
)

const (
	Chain = "op-geth"

	ServiceNameRPC        = "op-geth-rpc"
	ServiceNameTxPool     = "op-geth-txpool"
	ServiceNameMiner      = "op-geth-miner"
	ServiceNameState      = "op-geth-state"
	ServiceNameBlockchain = "op-geth-blockchain"

	Business = "op-geth"
	ChainID  = 0 // Will be set based on configuration

	Client               string = ""
	Status               string = ""
	Index                string = ""
	InnerIndex           string = ""
	ReferId              string = ""
	DepositConfirmHeight string = ""
	TokenID              string = ""
	MevSupplier          string = ""
	BusinessHash         string = ""
	ContractAddress      string = ""
)
