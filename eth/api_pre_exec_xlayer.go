package eth

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"strings"
	"time"

	coreState "github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/tracers"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/internal/ethapi/override"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/google/uuid"
)

const (
	UnKnownErrCode             = 1000
	InsufficientBalanceErrCode = 1001
	RevertedErrCode            = 1002
	CheckPreArgsErrCode        = 1003

	MaxGasLimit = 30000000
)

// PreExecInnerTx defines the structure for inner transactions returned by TransactionPreExec RPC
// This is specifically designed for the eth_transaction_preexec endpoint
type PreExecInnerTx struct {
	Dept          big.Int `json:"dept"`
	InternalIndex big.Int `json:"internal_index"`
	CallType      string  `json:"call_type"`
	Name          string  `json:"name"`
	TraceAddress  string  `json:"trace_address"`
	CodeAddress   string  `json:"code_address"`
	From          string  `json:"from"`
	To            string  `json:"to"`
	Input         string  `json:"input"`
	Output        string  `json:"output"`
	IsError       bool    `json:"is_error"`
	GasUsed       uint64  `json:"gas_used"`
	Value         string  `json:"value"`
	ValueWei      string  `json:"value_wei"`
	Error         string  `json:"error"`
	ReturnGas     uint64  `json:"return_gas"`
}

type PreArgs struct {
	ChainId              *big.Int                     `json:"chainId,omitempty"`
	From                 *common.Address              `json:"from"`
	To                   *common.Address              `json:"to"`
	Gas                  *hexutil.Uint64              `json:"gas"`
	GasPrice             *hexutil.Big                 `json:"gasPrice"`
	MaxFeePerGas         *hexutil.Big                 `json:"maxFeePerGas"`
	MaxPriorityFeePerGas *hexutil.Big                 `json:"maxPriorityFeePerGas"`
	Value                *hexutil.Big                 `json:"value"`
	Nonce                *hexutil.Uint64              `json:"nonce"`
	Data                 *hexutil.Bytes               `json:"data"`
	Input                *hexutil.Bytes               `json:"input"`
	AuthorizationList    []types.SetCodeAuthorization `json:"authorizationList"`
}

type CallTracerResult struct {
	Calls        []CallTracerResult `json:"calls"`
	From         string             `json:"from"`
	Gas          string             `json:"gas"`
	GasUsed      string             `json:"gasUsed"`
	Input        string             `json:"input"`
	Output       string             `json:"output,omitempty"`
	To           string             `json:"to"`
	Type         string             `json:"type"`
	Value        string             `json:"value"`
	Error        string             `json:"error,omitempty"`
	RevertReason string             `json:"revertReason,omitempty"`
}

type StateAccount struct {
	Balance string            `json:"balance"`
	Code    string            `json:"code"`
	Nonce   uint64            `json:"nonce"`
	Storage map[string]string `json:"storage"`
}

type PreError struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func (args PreArgs) ToLogString() string {
	argsBytes, _ := json.Marshal(args)
	return string(argsBytes)
}

func toPreError(err error, result *core.ExecutionResult) PreError {
	preErr := PreError{
		Code: UnKnownErrCode,
	}
	if err != nil {
		preErr.Msg = err.Error()
	}
	if result != nil && result.Err != nil {
		preErr.Msg = result.Err.Error()
	}
	if strings.HasPrefix(preErr.Msg, "execution reverted") {
		preErr.Code = RevertedErrCode
		if result != nil {
			preErr.Msg, _ = abi.UnpackRevert(result.Revert())
		}
	}
	if strings.HasPrefix(preErr.Msg, "out of gas") {
		preErr.Code = RevertedErrCode
	}
	if strings.HasPrefix(preErr.Msg, "insufficient funds for transfer") {
		preErr.Code = InsufficientBalanceErrCode
	}
	if strings.HasPrefix(preErr.Msg, "insufficient balance for transfer") {
		preErr.Code = InsufficientBalanceErrCode
	}
	if strings.HasPrefix(preErr.Msg, "insufficient funds for gas * price") {
		preErr.Code = InsufficientBalanceErrCode
	}
	return preErr
}

type PreResult struct {
	InnerTxs    interface{} `json:"innerTxs"`
	Logs        interface{} `json:"logs"`
	StateDiff   interface{} `json:"stateDiff"`
	Error       PreError    `json:"error"`
	GasUsed     uint64      `json:"gasUsed"`
	BlockNumber *big.Int    `json:"blockNumber"`
}

func (res PreResult) ToLogString() string {
	// spilt InnerTxs
	innerTxsStr, err := json.Marshal(res.InnerTxs)
	if err == nil {
		innerTxs := make([]*PreExecInnerTx, 0)
		if err := json.Unmarshal(innerTxsStr, &innerTxs); err == nil {
			if len(innerTxs) > 0 {
				innerTx := innerTxs[0]
				if len(innerTx.Input) > 100 {
					innerTx.Input = innerTx.Input[0:100]
				}
				if len(innerTx.Output) > 100 {
					innerTx.Output = innerTx.Output[0:100]
				}
				res.InnerTxs = append([]*PreExecInnerTx{}, innerTx)
			}
		}
	}
	// spilt Logs
	logsStr, err := json.Marshal(res.Logs)
	if err == nil {
		logs := make([]types.Log, 0)
		if err := json.Unmarshal(logsStr, &logs); err == nil {
			if len(logs) > 0 {
				l := logs[0]
				if len(l.Data) > 100 {
					l.Data = l.Data[0:100]
				}
				res.Logs = append([]types.Log{}, l)
			}
		}
	}
	resBytes, _ := json.Marshal(res)
	return string(resBytes)
}

func toPreResult(innerTxs []*PreExecInnerTx, logs []*types.Log, stateDiff map[string]interface{},
	preError PreError, gasUsed uint64, number *big.Int) PreResult {
	preResult := PreResult{
		Error:       preError,
		GasUsed:     gasUsed,
		BlockNumber: number,
	}
	if len(innerTxs) > 0 {
		preResult.InnerTxs = innerTxs
	} else {
		preResult.InnerTxs = make([]PreExecInnerTx, 0)
	}
	if len(logs) > 0 {
		preResult.Logs = logs
	} else {
		preResult.Logs = make([]types.Log, 0)
	}
	if len(stateDiff) > 0 {
		preResult.StateDiff = stateDiff
	} else {
		preResult.StateDiff = make(map[string]interface{}, 0)
	}

	return preResult
}

// TxPreExecAPI is the collection of Ethereum full node related APIs for transaction pre exec.
type TxPreExecAPI struct {
	eth *Ethereum
}

// NewTxPreExecAPI creates a new instance of TxPreExecAPI.
func NewTxPreExecAPI(eth *Ethereum) *TxPreExecAPI {
	return &TxPreExecAPI{eth: eth}
}

func (api *TxPreExecAPI) TransactionPreExec(ctx context.Context, origins []PreArgs, blockNrOrHash *rpc.BlockNumberOrHash, stateOverrides *override.StateOverride) ([]PreResult, error) {
	start := time.Now()
	// gen requestID
	requestID := uuid.NewString()
	defer func(s time.Time, id string) {
		log.Info("Executing TransactionPreExec call finished", "requestID", id, "runtime", time.Since(s))
	}(start, requestID)
	preResList := make([]PreResult, 0)

	bNrOrHash := rpc.BlockNumberOrHashWithNumber(rpc.LatestBlockNumber)
	if blockNrOrHash != nil {
		bNrOrHash = *blockNrOrHash
	}
	state, header, err := api.eth.APIBackend.StateAndHeaderByNumberOrHash(ctx, bNrOrHash)

	if state == nil || err != nil {
		return nil, err
	}
	if stateOverrides != nil {
		err = stateOverrides.Apply(state, nil)
		if err != nil {
			return nil, err
		}
	}
	blockNumber := big.NewInt(0).Set(header.Number)

	for i, origin := range origins {
		// Setup context with timeout for each individual transaction
		// ETH RPC EVM Timeout default 5s per transaction
		timeout := api.eth.APIBackend.RPCEVMTimeout()
		txCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		var gasUsed uint64
		log.Info("TransactionPreExec", "requestID", requestID, "input index", i, "input args", origin.ToLogString())

		var prevArg *PreArgs
		if i > 0 {
			prevArg = &origins[i-1]
		}
		correctedGas, err := preArgsCheck(state, origin, prevArg, i)
		if err != nil {
			preError := PreError{
				Code: CheckPreArgsErrCode,
				Msg:  err.Error(),
			}
			preResult := toPreResult(nil, nil, nil, preError, gasUsed, blockNumber)
			preResList = append(preResList, preResult)
			continue
		}

		if correctedGas != nil {
			origin.Gas = correctedGas
		}

		rawRes, gasUsed, receipt, err := applyMessageWithTracer(txCtx, api, state, origin, header, i, timeout)
		if err != nil {
			log.Error("TransactionPreExec: applyMessageWithTracer failed", "requestID", requestID, "input args", origin.ToLogString(), "error", err.Error())
			preError := toPreError(err, nil)
			preResult := toPreResult(nil, nil, nil, preError, gasUsed, blockNumber)
			preResList = append(preResList, preResult)
			continue
		}

		txHash := common.BigToHash(big.NewInt(int64(i)))

		preRes, err := processTracerResults(rawRes, state, txHash, header, receipt, gasUsed, blockNumber)
		if err != nil {
			log.Error("TransactionPreExec: processTracerResults failed", "requestID", requestID, "input args", origin.ToLogString(), "error", err.Error())
			preError := toPreError(err, nil)
			preResult := toPreResult(nil, nil, nil, preError, gasUsed, blockNumber)
			preResList = append(preResList, preResult)
			continue
		}

		preResList = append(preResList, preRes)
		log.Info("TransactionPreExec execute finished", "requestID", requestID, "input index", i, "result", preRes.ToLogString(), "runtime", time.Since(start))
	}
	return preResList, nil
}

func convertCallTracerResultToInnerTxs(traceResult interface{}) (result []*PreExecInnerTx, err error) {
	if traceResult == nil {
		return nil, fmt.Errorf("call tracer result is nil")
	}

	traceResultStr, err := json.Marshal(traceResult)
	if err != nil {
		return nil, err
	}
	callTx := CallTracerResult{}
	if err := json.Unmarshal(traceResultStr, &callTx); err != nil {
		return nil, err
	}

	result = make([]*PreExecInnerTx, 0)
	convertCallToInnerTxsRecursive(callTx, 0, 0, "", false, &result)
	return
}

// convertCallToInnerTxsRecursive recursively converts a CallTracerResult and all its nested calls to PreExecInnerTx
func convertCallToInnerTxsRecursive(callTx CallTracerResult, depth int64, index int64, depthIndexRoot string, parentError bool, result *[]*PreExecInnerTx) {
	isError := parentError
	var errorMsg string
	if callTx.Error != "" {
		isError = true
		errorMsg = callTx.Error
	}
	if callTx.Error != "" && callTx.RevertReason != "" {
		isError = true
		errorMsg = fmt.Sprintf("%s,%s", callTx.Error, callTx.RevertReason)
	}

	gasUsed := new(big.Int)
	if len(callTx.GasUsed) > 2 && strings.HasPrefix(callTx.GasUsed, "0x") {
		gasUsed, _ = gasUsed.SetString(callTx.GasUsed[2:], 16)
	}

	gas := new(big.Int)
	if len(callTx.Gas) > 2 && strings.HasPrefix(callTx.Gas, "0x") {
		gas, _ = gas.SetString(callTx.Gas[2:], 16)
	}

	valueWei := ""
	if len(callTx.Value) > 2 && strings.HasPrefix(callTx.Value, "0x") {
		valueWeiInt := new(big.Int)
		valueWeiInt, _ = valueWeiInt.SetString(callTx.Value[2:], 16)
		valueWei = valueWeiInt.String()
	}

	gasUint64 := gas.Uint64()
	gasUsedUint64 := gasUsed.Uint64()
	returnGas := uint64(0)
	if gasUint64 > gasUsedUint64 {
		returnGas = gasUint64 - gasUsedUint64
	}

	// Handle empty output - ensure it's "0x" instead of ""
	output := callTx.Output
	if output == "" {
		output = "0x"
	}

	// Create inner transaction
	innerTx := &PreExecInnerTx{
		Dept:          *big.NewInt(depth),
		InternalIndex: *big.NewInt(index),
		CallType:      strings.ToLower(callTx.Type),
		TraceAddress:  "",
		CodeAddress:   "",
		From:          common.HexToAddress(callTx.From).Hex(),
		To:            common.HexToAddress(callTx.To).Hex(),
		Input:         callTx.Input,
		Output:        output,
		IsError:       isError,
		GasUsed:       gasUsedUint64,
		Value:         valueWei,
		ValueWei:      valueWei,
		Error:         errorMsg,
		ReturnGas:     returnGas,
	}

	// Handle root vs nested call differences
	isRoot := depth == 0
	if isRoot {
		innerTx.Name = strings.ToLower(callTx.Type)
	} else {
		if len(callTx.From) > 2 && strings.HasPrefix(callTx.From, "0x") {
			innerTx.From = "0x000000000000000000000000" + callTx.From[2:]
		}
		if len(callTx.To) > 2 && strings.HasPrefix(callTx.To, "0x") {
			innerTx.To = "0x000000000000000000000000" + callTx.To[2:]
		}

		if strings.ToLower(callTx.Type) == "callcode" {
			innerTx.CodeAddress = callTx.To
		}

		currentDepthIndexRoot := fmt.Sprintf("%s_%d", depthIndexRoot, index)
		innerTx.Name = fmt.Sprintf("%s%s", innerTx.CallType, currentDepthIndexRoot)
	}

	// Add current transaction to result
	*result = append(*result, innerTx)

	// Recursively process nested calls
	if len(callTx.Calls) > 0 {
		for i, nestedCall := range callTx.Calls {
			var nestedDepthIndexRoot string
			if isRoot {
				nestedDepthIndexRoot = ""
			} else {
				nestedDepthIndexRoot = fmt.Sprintf("%s_%d", depthIndexRoot, index)
			}
			convertCallToInnerTxsRecursive(nestedCall, depth+1, int64(i), nestedDepthIndexRoot, innerTx.IsError, result)
		}
	}
}

func convertPrestateTracerResultToStateDiff(traceResult interface{}) (result map[string]interface{}, err error) {
	if traceResult == nil {
		return nil, fmt.Errorf("prestate tracer result is nil")
	}
	result = make(map[string]interface{})
	stateDiffResultStr, err := json.Marshal(traceResult)
	if err != nil {
		return nil, err
	}
	stateAccount := make(map[string]map[common.Address]*StateAccount)
	if err := json.Unmarshal(stateDiffResultStr, &stateAccount); err != nil {
		return nil, err
	}
	var pre, post map[common.Address]*StateAccount
	if preState, exist := stateAccount["pre"]; exist {
		pre = preState
	} else {
		return nil, nil
	}
	if postState, exist := stateAccount["post"]; exist {
		post = postState
	} else {
		return nil, nil
	}

	for addr, postState := range post {
		if preState, exist := pre[addr]; exist {
			addrMap := make(map[string]interface{})
			preStateBalance, postStateBalance := new(big.Int), new(big.Int)
			if strings.HasPrefix(preState.Balance, "0x") {
				preStateBalance, _ = big.NewInt(0).SetString(preState.Balance[2:], 16)
			}
			// post state balance
			if strings.HasPrefix(postState.Balance, "0x") {
				postStateBalance, _ = big.NewInt(0).SetString(postState.Balance[2:], 16)
			} else {
				postStateBalance = preStateBalance
			}
			balance := struct {
				Before string `json:"before"`
				After  string `json:"after"`
			}{
				Before: preStateBalance.String(),
				After:  postStateBalance.String(),
			}
			addrMap["balance"] = balance
			result[addr.String()] = addrMap
		}
	}

	return
}

func applyMessageWithTracer(ctx context.Context, api *TxPreExecAPI, state *coreState.StateDB, origin PreArgs, header *types.Header, index int, timeout time.Duration) (json.RawMessage, uint64, *types.Receipt, error) {
	// get ChainID from ChainConfig
	chainId := api.eth.APIBackend.ChainConfig().ChainID
	txArgs := ethapi.TransactionArgs{
		ChainID:              (*hexutil.Big)(chainId),
		From:                 origin.From,
		To:                   origin.To,
		Gas:                  origin.Gas,
		GasPrice:             origin.GasPrice,
		MaxFeePerGas:         origin.MaxFeePerGas,
		MaxPriorityFeePerGas: origin.MaxPriorityFeePerGas,
		Value:                origin.Value,
		Data:                 origin.Data,
		Input:                origin.Input,
		AuthorizationList:    origin.AuthorizationList,
	}

	if err := txArgs.CallDefaults(api.eth.APIBackend.RPCGasCap(), header.BaseFee, api.eth.APIBackend.ChainConfig().ChainID); err != nil {
		return nil, 0, nil, err
	}

	msg := txArgs.ToMessage(header.BaseFee, true, true)
	tx := txArgs.ToTransaction(types.LegacyTxType)

	txHash := common.BigToHash(big.NewInt(int64(index)))
	traceConfig := []byte(`
		{
			"prestateTracer": {
				"diffMode": true
			},
			"callTracer": null
		}
	`)

	txctx := &tracers.Context{
		BlockHash:   header.Hash(),
		BlockNumber: big.NewInt(0).Set(header.Number),
		TxIndex:     0,
		TxHash:      txHash,
	}
	tracer, err := tracers.DefaultDirectory.New("muxTracer", txctx, traceConfig, api.eth.APIBackend.ChainConfig())
	if err != nil {
		return nil, 0, nil, err
	}

	hookedState := coreState.NewHookedState(state, tracer.Hooks)
	blockContext := core.NewEVMBlockContext(header, api.eth.BlockChain(), nil, api.eth.APIBackend.ChainConfig(), state)
	evm := vm.NewEVM(blockContext, hookedState, api.eth.APIBackend.ChainConfig(), vm.Config{NoBaseFee: true, Tracer: tracer.Hooks})

	// Setup timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	go func() {
		<-timeoutCtx.Done()
		evm.Cancel()
	}()

	evm.Context.BaseFee = big.NewInt(0)

	// blocknumber and time are random to simulate real transactions with propogation delay
	evm.Context.BlockNumber.Add(evm.Context.BlockNumber, big.NewInt(rand.Int63n(6)+6))
	evm.Context.Time += uint64(rand.Int63n(60) + 30)
	gp := new(core.GasPool).AddGas(MaxGasLimit)
	state.SetTxContext(txHash, index)

	var gasUsed uint64
	receipt, err := core.ApplyTransactionWithEVM(msg, gp, state, evm.Context.BlockNumber, txctx.BlockHash, tx, &gasUsed, evm)
	if err != nil {
		return nil, gasUsed, nil, err
	}

	// If the timer caused an abort, return an appropriate error message
	if evm.Cancelled() {
		return nil, gasUsed, nil, fmt.Errorf("execution aborted (timeout = %v)", timeout)
	}

	rawRes, err := tracer.GetResult()
	if err != nil {
		return nil, gasUsed, nil, err
	}

	return rawRes, gasUsed, receipt, nil
}

func processTracerResults(rawRes json.RawMessage, state *coreState.StateDB, txHash common.Hash, header *types.Header, receipt *types.Receipt, gasUsed uint64, blockNumber *big.Int) (PreResult, error) {
	var res map[string]interface{}
	if err := json.Unmarshal(rawRes, &res); err != nil {
		preError := toPreError(err, nil)
		return toPreResult(nil, nil, nil, preError, gasUsed, blockNumber), nil
	}

	// convert callTracer result to inner txs
	innerTxs := make([]*PreExecInnerTx, 0)
	if t, exist := res["callTracer"]; exist {
		convertedInnerTxs, err := convertCallTracerResultToInnerTxs(t)
		if err != nil {
			preError := PreError{
				Code: UnKnownErrCode,
				Msg:  err.Error(),
			}
			return toPreResult(nil, nil, nil, preError, gasUsed, blockNumber), nil
		}
		// innerTxs are set only when there are deep calls (contract to contract calls) or failed calls
		hasDeepCalls := false
		hasFailedCalls := false
		for _, innerTx := range convertedInnerTxs {
			if innerTx.Dept.Int64() > 0 {
				hasDeepCalls = true
			}
			if innerTx.IsError || innerTx.Error != "" {
				hasFailedCalls = true
			}
		}
		if hasDeepCalls || hasFailedCalls {
			innerTxs = convertedInnerTxs
		}
	}

	// convert prestateTracer result to state diff
	stateDiff := make(map[string]interface{}, 0)
	if t, exist := res["prestateTracer"]; exist {
		var err error
		stateDiff, err = convertPrestateTracerResultToStateDiff(t)
		if err != nil {
			preError := PreError{
				Code: UnKnownErrCode,
				Msg:  err.Error(),
			}
			return toPreResult(nil, nil, nil, preError, gasUsed, blockNumber), nil
		}
	}

	// Create the final result
	preRes := toPreResult(innerTxs, state.GetLogs(txHash, header.Number.Uint64(), header.Hash()), stateDiff, PreError{}, gasUsed, blockNumber)

	// Handle receipt status failures
	if receipt != nil && receipt.Status == types.ReceiptStatusFailed {
		preRes.Error = toPreError(nil, nil)
	}

	// Handle inner transaction errors
	if preRes.Error.Msg == "" && len(innerTxs) != 0 && innerTxs[0].Error != "" {
		preRes.Error = PreError{
			Code: RevertedErrCode,
			Msg:  innerTxs[0].Error,
		}
	}

	return preRes, nil
}

func preArgsCheck(state *coreState.StateDB, arg PreArgs, prevArg *PreArgs, index int) (*hexutil.Uint64, error) {
	if arg.From == nil {
		return nil, fmt.Errorf("from is nil")
	}

	if arg.To == nil {
		return nil, fmt.Errorf("to is nil")
	}

	if arg.Nonce == nil {
		return nil, fmt.Errorf("%s, nonce is nil", arg.From.Hex())
	}

	// check whether sender's nonce decreases
	if prevArg != nil && *arg.From == *prevArg.From && (uint64)(*arg.Nonce) <= (uint64)(*prevArg.Nonce) {
		return nil, fmt.Errorf("%v nonce decreases, tx index %d has nonce %d, tx index %d has nonce %d",
			arg.From.Hex(), index-1, (uint64)(*prevArg.Nonce), index, (uint64)(*arg.Nonce))
	}

	msgFrom := *arg.From
	msgNonce := uint64(*arg.Nonce)
	stNonce := state.GetNonce(msgFrom)

	if stNonce > msgNonce {
		return nil, fmt.Errorf("%w: address %v, tx: %d state: %d", core.ErrNonceTooLow,
			msgFrom.Hex(), msgNonce, stNonce)
	} else if stNonce+1 < stNonce {
		return nil, fmt.Errorf("%w: address %v, nonce: %d", core.ErrNonceMax,
			msgFrom.Hex(), stNonce)
	}

	// check gas, if gas value is 0 or > 30000000, return corrected gas value
	if arg.Gas == nil {
		gas := uint64(MaxGasLimit)
		return (*hexutil.Uint64)(&gas), nil
	} else {
		gas := uint64(*arg.Gas)
		if gas == 0 || gas > uint64(MaxGasLimit) {
			gas = uint64(MaxGasLimit)
			return (*hexutil.Uint64)(&gas), nil
		}
	}

	return nil, nil
}
