package operations

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

// normal jsonrpc message
type jsonrpcMessage struct {
	Version string          `json:"jsonrpc,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Error   *jsonError      `json:"error,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
}

type jsonError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

/**
	This file aims to create a mock of erigon server for testing purposes.
**/

// 1. Create a mock HTTP server that simulates an Erigon RPC endpoint
// 2. Create a mock Erigon RPC service
// 3. Register the mock Erigon RPC service to the mock HTTP server
// 4. Run the mock HTTP server in a separate goroutine

// CreateMockErigonServer creates a mock Erigon server with a specific port
func CreateMockErigonServer() *httptest.Server {
	return createMockErigonServerInternal()
}

// createMockErigonServerInternal is the internal implementation
func createMockErigonServerInternal() *httptest.Server {
	// Create a handler map for extensibility - new methods can be easily added here
	handlers := map[string]func(params json.RawMessage) (json.RawMessage, error){
		"eth_chainId": handleEthChainId,
	}

	// Create HTTP handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Parse the JSON-RPC request
		var req jsonrpcMessage
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response := jsonrpcMessage{
				Version: "2.0",
				ID:      req.ID,
				Error: &jsonError{
					Code:    -32700,
					Message: "Parse error",
				},
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		// Find the handler for the requested method
		methodHandler, ok := handlers[req.Method]
		if !ok {
			response := jsonrpcMessage{
				Version: "2.0",
				ID:      req.ID,
				Error: &jsonError{
					Code:    -32601,
					Message: "Method not found",
				},
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		// Execute the handler
		result, err := methodHandler(req.Params)
		if err != nil {
			response := jsonrpcMessage{
				Version: "2.0",
				ID:      req.ID,
				Error: &jsonError{
					Code:    -32000,
					Message: err.Error(),
				},
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		// Send success response
		response := jsonrpcMessage{
			Version: "2.0",
			ID:      req.ID,
			Result:  result,
		}
		json.NewEncoder(w).Encode(response)
	})

	// Create the mock HTTP server
	var server *httptest.Server

	server = httptest.NewUnstartedServer(handler)
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", DefaultErigonRPCPort))
	if err != nil {
		panic(fmt.Sprintf("Failed to create listener on port %d: %v", DefaultErigonRPCPort, err))
	}
	server.Listener = listener
	server.Start()

	return server
}

// handleEthChainId handles the eth_chainId RPC method
// Returns chain ID "0x1" (mainnet)
func handleEthChainId(params json.RawMessage) (json.RawMessage, error) {
	// Return "0x1" as a JSON string
	ethChainIdHex := hexutil.EncodeUint64(DefaultL2ChainID)
	return json.RawMessage(`"` + ethChainIdHex + `"`), nil
}
