package vm

import (
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

const (
	CALL_TYP         = "call"
	CALLCODE_TYP     = "callcode"
	DELEGATECALL_TYP = "delegatecall"
	STATICCAL_TYP    = "staticcall"
	CREATE_TYP       = "create"
	CREATE2_TYP      = "create2"
	SUICIDE_TYP      = "suicide"
)

type InnerTxMeta struct {
	index     int
	lastDepth int
	indexMap  map[int]int
	InnerTxs  []*types.InnerTx
}

func (evm *EVM) GetInnerTxMeta() *InnerTxMeta {
	return evm.innerTxMeta
}

func (evm *EVM) AddInnerTx(innerTx *types.InnerTx) {
	evm.innerTxMeta.InnerTxs = append(evm.innerTxMeta.InnerTxs, innerTx)
}

func (evm *EVM) PopInnerTxs() []*types.InnerTx {
	innertxs := make([]*types.InnerTx, len(evm.innerTxMeta.InnerTxs))
	copy(innertxs, evm.innerTxMeta.InnerTxs)

	evm.innerTxMeta.InnerTxs = evm.innerTxMeta.InnerTxs[:0]
	evm.innerTxMeta.index = 0
	evm.innerTxMeta.lastDepth = 0
	// Clear the index map
	for k := range evm.innerTxMeta.indexMap {
		delete(evm.innerTxMeta.indexMap, k)
	}
	return innertxs
}

func beforeOp(
	interpreter *EVMInterpreter,
	callTyp string,
	fromAddr common.Address,
	toAddr *common.Address,
	codeAddr *common.Address,
	input []byte,
	gas uint64,
	value *big.Int) (*types.InnerTx, int) {
	innerTx := &types.InnerTx{
		CallType:     callTyp,
		From:         fromAddr.String(),
		ValueWei:     value.String(),
		CallValueWei: hexutil.EncodeBig(value),
		Gas:          gas,
		IsError:      false,
	}

	if toAddr != nil {
		innerTx.To = toAddr.String()
	}
	if codeAddr != nil {
		innerTx.CodeAddress = codeAddr.String()
	}

	if input != nil {
		innerTx.Input = hexutil.Encode(input)
	}

	innerTxMeta := interpreter.evm.GetInnerTxMeta()
	depth := interpreter.evm.depth
	if depth == innerTxMeta.lastDepth {
		innerTxMeta.index++
		innerTxMeta.indexMap[depth] = innerTxMeta.index
	} else if depth < innerTxMeta.lastDepth {
		innerTxMeta.index = innerTxMeta.indexMap[depth] + 1
		innerTxMeta.indexMap[depth] = innerTxMeta.index
		innerTxMeta.lastDepth = depth
	} else if depth > innerTxMeta.lastDepth {
		innerTxMeta.index = 0
		innerTxMeta.indexMap[depth] = 0
		innerTxMeta.lastDepth = depth
	}
	for i := 1; i <= innerTxMeta.lastDepth; i++ {
		innerTx.Name = innerTx.Name + "_" + strconv.Itoa(innerTxMeta.indexMap[i])
	}
	innerTx.Name = innerTx.CallType + innerTx.Name
	innerTx.Dept = *big.NewInt(int64(depth))
	innerTx.InternalIndex = *big.NewInt(int64(innerTxMeta.index))

	interpreter.evm.AddInnerTx(innerTx)

	newIndex := len(interpreter.evm.GetInnerTxMeta().InnerTxs) - 1
	if newIndex < 0 {
		newIndex = 0
	}

	return innerTx, newIndex
}

func afterOp(interpreter *EVMInterpreter, opType string, gas_used uint64, newIndex int, innerTx *types.InnerTx, addr *common.Address, err error, ret []byte) {
	innerTx.GasUsed = gas_used
	if ret != nil {
		innerTx.Output = hexutil.Encode(ret[:])
	}
	if err != nil {
		innerTx.Error = err.Error()
		innerTx.IsError = true
		innerTxMeta := interpreter.evm.GetInnerTxMeta()
		for _, tx := range innerTxMeta.InnerTxs[newIndex:] {
			tx.IsError = true
		}
	}

	switch opType {
	case CREATE_TYP, CREATE2_TYP:
		if addr != nil {
			innerTx.To = addr.String()
		}
	}
}
