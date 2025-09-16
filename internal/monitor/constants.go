package monitor

// ProcessStep represents a step in transaction processing
type ProcessStep struct {
	ID  uint64
	Key string
}

var (
	// RPC Service Steps
	StepRPCReceiveTx    = ProcessStep{10010, "rpc_receive_tx"}
	StepRPCSendTx       = ProcessStep{10012, "rpc_send_tx"}
	StepRPCReceiveBlock = ProcessStep{10060, "rpc_receive_block"}
	StepRPCFinishBlock  = ProcessStep{10062, "rpc_finish_block"}

	// Transaction Pool Steps
	StepTxPoolAdd      = ProcessStep{10020, "txpool_add"}
	StepTxPoolValidate = ProcessStep{10022, "txpool_validate"}
	StepTxPoolAccept   = ProcessStep{10024, "txpool_accept"}
	StepTxPoolReject   = ProcessStep{10026, "txpool_reject"}

	// Mining Steps
	StepMinerSelectTx  = ProcessStep{10030, "miner_select_tx"}
	StepMinerExecuteTx = ProcessStep{10032, "miner_execute_tx"}
	StepMinerPackageTx = ProcessStep{10034, "miner_package_tx"}
	StepMinerEndBlock  = ProcessStep{10036, "miner_end_block"}

	// State Processing Steps
	StepStateProcessTx       = ProcessStep{10040, "state_process_tx"}
	StepStateApplyTx         = ProcessStep{10042, "state_apply_tx"}
	StepStateGenerateReceipt = ProcessStep{10044, "state_generate_receipt"}
	StepStateCommit          = ProcessStep{10046, "state_commit"}

	// Blockchain Steps
	StepBlockchainInsert   = ProcessStep{10050, "blockchain_insert"}
	StepBlockchainValidate = ProcessStep{10052, "blockchain_validate"}
	StepBlockchainFinalize = ProcessStep{10054, "blockchain_finalize"}
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
