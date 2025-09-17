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

func (args PreArgs) ToLogString() string {
	argsBytes, _ := json.Marshal(args)
	return string(argsBytes)
}

type PreError struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
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

	// Setup context so it may be cancelled the call has completed
	// or, in case of unmetered gas, setup a context with a timeout.
	var cancel context.CancelFunc
	// ETH RPC EVM Timeout default 5s
	timeout := api.eth.APIBackend.RPCEVMTimeout()
	if len(origins) > 0 {
		timeout = time.Duration(len(origins)) * timeout
	}
	ctx, cancel = context.WithTimeout(ctx, timeout)
	// Make sure the context is cancelled when the call has completed
	// this makes sure resources are cleaned up.
	defer cancel()

	for i := 0; i < len(origins); i++ {
		var gasUsed uint64
		origin := origins[i]
		log.Info("TransactionPreExec", "requestID", requestID, "input index", i, "input args", origin.ToLogString())
		// check pre args
		if err := preArgsCheck(state, origin); err != nil {
			preError := PreError{
				Code: CheckPreArgsErrCode,
				Msg:  err.Error(),
			}
			preResult := toPreResult(nil, nil, nil, preError, gasUsed, blockNumber)
			preResList = append(preResList, preResult)
			continue
		}
		// check whether sender's nonce decreases
		if i > 0 && *origin.From == *origins[i-1].From && (uint64)(*origin.Nonce) <= (uint64)(*origins[i-1].Nonce) {
			preError := PreError{
				Code: CheckPreArgsErrCode,
				Msg:  fmt.Sprintf("%v nonce decreases, tx index %d has nonce %d, tx index %d has nonce %d", origin.From.Hex(), i-1, (uint64)(*origins[i-1].Nonce), i, (uint64)(*origin.Nonce)),
			}
			preResult := toPreResult(nil, nil, nil, preError, gasUsed, blockNumber)
			preResList = append(preResList, preResult)
			continue
		}
		// check gas, if gas value is 0 or > 30000000, set it to 30000000
		if origin.Gas == nil {
			gas := uint64(MaxGasLimit)
			origin.Gas = (*hexutil.Uint64)(&gas)
		} else {
			gas := uint64(*origin.Gas)
			if gas == 0 || gas > uint64(MaxGasLimit) {
				gas = uint64(MaxGasLimit)
				origin.Gas = (*hexutil.Uint64)(&gas)
			}
		}
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
			log.Error("TransactionPreExec: tx args call defaults failed", "requestID", requestID, "input args", origin.ToLogString(), "error", err.Error())
			preError := PreError{
				Code: CheckPreArgsErrCode,
				Msg:  err.Error(),
			}
			preResult := toPreResult(nil, nil, nil, preError, gasUsed, blockNumber)
			preResList = append(preResList, preResult)
			continue
		}
		msg := txArgs.ToMessage(header.BaseFee, true, true)
		tx := txArgs.ToTransaction(types.LegacyTxType)

		txHash := common.BigToHash(big.NewInt(int64(i)))
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
			log.Error("TransactionPreExec: generate muxTracer failed", "requestID", requestID, "input args", origin.ToLogString(), "error", err.Error())
			return nil, err
		}

		hookedState := coreState.NewHookedState(state, tracer.Hooks)
		blockContext := core.NewEVMBlockContext(header, api.eth.BlockChain(), nil, api.eth.APIBackend.ChainConfig(), state)
		evm := vm.NewEVM(blockContext, hookedState, api.eth.APIBackend.ChainConfig(), vm.Config{NoBaseFee: true, Tracer: tracer.Hooks})

		go func() {
			<-ctx.Done()
			evm.Cancel()
		}()
		evm.Context.BaseFee = big.NewInt(0)
		evm.Context.BlockNumber.Add(evm.Context.BlockNumber, big.NewInt(rand.Int63n(6)+6))
		evm.Context.Time += uint64(rand.Int63n(60) + 30)
		gp := new(core.GasPool).AddGas(MaxGasLimit)
		state.SetTxContext(txHash, i)

		receipt, err := core.ApplyTransactionWithEVM(msg, gp, state, evm.Context.BlockNumber, txctx.BlockHash, tx, &gasUsed, evm)
		if err != nil {
			log.Error("TransactionPreExec: core apply transaction with evm failed", "requestID", requestID, "input args", origin.ToLogString(), "error", err.Error())
			preError := toPreError(err, nil)
			preResult := toPreResult(nil, nil, nil, preError, gasUsed, blockNumber)
			preResList = append(preResList, preResult)
			continue
		}
		// If the timer caused an abort, return an appropriate error message
		if evm.Cancelled() {
			log.Error("TransactionPreExec: evm execution aborted timeout", "requestID", requestID, "input args", origin.ToLogString())
			preError := PreError{
				Code: UnKnownErrCode,
				Msg:  fmt.Sprintf("execution aborted (timeout = %v)", timeout),
			}
			preResult := toPreResult(nil, nil, nil, preError, gasUsed, blockNumber)
			preResList = append(preResList, preResult)
			continue
		}
		rawRes, err := tracer.GetResult()
		if err != nil {
			log.Error("TransactionPreExec: tracer get result failed", "requestID", requestID, "input args", origin.ToLogString(), "error", err.Error())
			preError := toPreError(err, nil)
			preResult := toPreResult(nil, nil, nil, preError, gasUsed, blockNumber)
			preResList = append(preResList, preResult)
			continue
		}
		var res map[string]interface{}
		if err := json.Unmarshal(rawRes, &res); err != nil {
			preError := toPreError(err, nil)
			preResult := toPreResult(nil, nil, nil, preError, gasUsed, blockNumber)
			preResList = append(preResList, preResult)
			continue
		}
		// convert callTracer result to inner txs
		innerTxs := make([]*PreExecInnerTx, 0)
		if t, exist := res["callTracer"]; exist {
			convertedInnerTxs, err := convertCallTracerResultToInnerTxs(t)
			if err != nil {
				log.Error("TransactionPreExec: convert CallTracer result to innerTxs failed", "requestID", requestID, "input args", origin.ToLogString(), "error", err.Error())
				preError := PreError{
					Code: UnKnownErrCode,
					Msg:  err.Error(),
				}
				preResult := toPreResult(nil, nil, nil, preError, gasUsed, blockNumber)
				preResList = append(preResList, preResult)
				continue
			} else {
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

		}
		// convert prestateTracer result to state diff
		stateDiff := make(map[string]interface{}, 0)
		if t, exist := res["prestateTracer"]; exist {
			stateDiff, err = convertPrestateTracerResultToStateDiff(t)
			if err != nil {
				log.Error("TransactionPreExec: convert PrestateTracer result to stateDiff failed", "requestID", requestID, "input args", origin.ToLogString(), "error", err.Error())
				preError := PreError{
					Code: UnKnownErrCode,
					Msg:  err.Error(),
				}
				preResult := toPreResult(nil, nil, nil, preError, gasUsed, blockNumber)
				preResList = append(preResList, preResult)
				continue
			}
		}

		preRes := toPreResult(innerTxs, state.GetLogs(txHash, header.Number.Uint64(), header.Hash()), stateDiff, PreError{}, gasUsed, blockNumber)
		if receipt != nil && receipt.Status == types.ReceiptStatusFailed {
			preRes.Error = toPreError(err, nil)
		}
		if preRes.Error.Msg == "" && len(innerTxs) != 0 && innerTxs[0].Error != "" {
			preRes.Error = PreError{
				Code: RevertedErrCode,
				Msg:  innerTxs[0].Error,
			}
		}
		preResList = append(preResList, preRes)
		log.Info("TransactionPreExec execute finished", "requestID", requestID, "input index", i, "result", preRes.ToLogString(), "runtime", time.Since(start))
	}
	return preResList, nil
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
	isError := false
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

	valueWei := ""
	if len(callTx.Value) > 2 && strings.HasPrefix(callTx.Value, "0x") {
		valueWeiInt := new(big.Int)
		valueWeiInt, _ = valueWeiInt.SetString(callTx.Value[2:], 16)
		valueWei = valueWeiInt.String()
	}

	gas := new(big.Int)
	if len(callTx.Gas) > 2 && strings.HasPrefix(callTx.Gas, "0x") {
		gas, _ = gas.SetString(callTx.Gas[2:], 16)
	}

	// Calculate ReturnGas = Gas - GasUsed
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

	innerTx := &PreExecInnerTx{
		Dept:          *big.NewInt(0),
		InternalIndex: *big.NewInt(int64(0)),
		CallType:      strings.ToLower(callTx.Type),
		Name:          strings.ToLower(callTx.Type),
		TraceAddress:  "",
		CodeAddress:   "",
		From:          common.HexToAddress(callTx.From).Hex(), // Convert to checksummed address
		To:            common.HexToAddress(callTx.To).Hex(),   // Convert to checksummed address
		Input:         callTx.Input,
		Output:        output, // Use processed output
		IsError:       isError,
		GasUsed:       gasUsedUint64, // For historical reason, we use gasUint64 here
		Value:         valueWei,
		ValueWei:      valueWei,
		Error:         errorMsg,
		ReturnGas:     returnGas,
	}
	result = append(result, innerTx)
	if len(callTx.Calls) > 0 {
		// convert calls to innerTxs
		callInnerTxs := convertCallsToInnerTxs(callTx.Calls, 0, "", isError)
		result = append(result, callInnerTxs...)
	}

	return
}

func convertCallsToInnerTxs(calls []CallTracerResult, lastDepth int64, lastDepthIndexRoot string, isError bool) (result []*PreExecInnerTx) {
	result = make([]*PreExecInnerTx, 0)
	depth := lastDepth + 1
	for index, callTx := range calls {
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

		// Calculate ReturnGas = Gas - GasUsed
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

		innerTx := &PreExecInnerTx{
			Dept:          *big.NewInt(depth),
			InternalIndex: *big.NewInt(int64(index)),
			CallType:      strings.ToLower(callTx.Type),
			Name:          "",
			TraceAddress:  "",
			CodeAddress:   "",
			From:          common.HexToAddress(callTx.From).Hex(), // Convert to checksummed address
			To:            common.HexToAddress(callTx.To).Hex(),   // Convert to checksummed address
			Input:         callTx.Input,
			Output:        output, // Use processed output
			IsError:       isError,
			GasUsed:       gasUsedUint64,
			Value:         valueWei,
			ValueWei:      valueWei,
			Error:         errorMsg,
			ReturnGas:     returnGas,
		}
		// convert from/ to address
		if len(callTx.From) > 2 && strings.HasPrefix(callTx.From, "0x") {
			innerTx.From = "0x000000000000000000000000" + callTx.From[2:]
		}

		if len(callTx.To) > 2 && strings.HasPrefix(callTx.To, "0x") {
			innerTx.To = "0x000000000000000000000000" + callTx.To[2:]
		}

		// if CallType == "callcode",  CodeAddress = callTx.To
		if strings.ToLower(callTx.Type) == "callcode" {
			innerTx.CodeAddress = callTx.To
		}

		depthIndexRoot := fmt.Sprintf("%s_%d", lastDepthIndexRoot, index)
		innerTx.Name = fmt.Sprintf("%s%s", innerTx.CallType, depthIndexRoot)

		result = append(result, innerTx)
		if len(callTx.Calls) > 0 {
			innerTxs := convertCallsToInnerTxs(callTx.Calls, depth, depthIndexRoot, isError)
			result = append(result, innerTxs...)
		}
	}
	return result
}

type StateAccount struct {
	Balance string            `json:"balance"`
	Code    string            `json:"code"`
	Nonce   uint64            `json:"nonce"`
	Storage map[string]string `json:"storage"`
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

func preArgsCheck(state *coreState.StateDB, arg PreArgs) error {
	if arg.From == nil {
		return fmt.Errorf("from is nil")
	}

	if arg.To == nil {
		return fmt.Errorf("to is nil")
	}

	if arg.Nonce == nil {
		return fmt.Errorf("%s, nonce is nil", arg.From.Hex())
	}

	msgFrom := *arg.From
	msgNonce := uint64(*arg.Nonce)
	stNonce := state.GetNonce(msgFrom)

	if stNonce > msgNonce {
		return fmt.Errorf("%w: address %v, tx: %d state: %d", core.ErrNonceTooLow,
			msgFrom.Hex(), msgNonce, stNonce)
	} else if stNonce+1 < stNonce {
		return fmt.Errorf("%w: address %v, nonce: %d", core.ErrNonceMax,
			msgFrom.Hex(), stNonce)
	}

	return nil
}
