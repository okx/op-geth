package apollo

const (
	// Component name prefixes for validation
	OpGethComponent = "opgeth"
	OpNodeComponent = "opnode"

	// Namespaces with component prefixes (using underscore to avoid splitting issues)
	// JsonRPC is the jsonrpc prefix namespace, the content of the prefix is the configuration for jsonrpc with yaml format
	JsonRPC = "jsonrpc"
	// Sequencer is the sequencer namespace for op-node, the content is the configuration for sequencer with yaml format
	Sequencer = "opnode_sequencer"
	// L2GasPricer is the l2gaspricer namespace for op-geth, the content is the configuration for l2gaspricer with yaml format
	L2GasPricer = "opgeth_l2gaspricer"
	// Pool is the pool namespace for op-geth, the content is the configuration for pool with yaml format
	Pool = "opgeth_pool"
	// Halt is the halt suffix namespace. Change the halt to a different value will halt the respective service
	Halt = "halt"
)
