package operations

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func CreateMockErigonServer() *httptest.Server {
	// Create a handler map for extensibility - new methods can be easily added here
	handlers := map[string]func(params json.RawMessage) (json.RawMessage, error){
		"eth_chainId": handleEthChainId,
	}

	// Create and return the mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		handler, ok := handlers[req.Method]
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
		result, err := handler(req.Params)
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
	}))

	return server
}

// handleEthChainId handles the eth_chainId RPC method
// Returns chain ID "0x1" (mainnet)
func handleEthChainId(params json.RawMessage) (json.RawMessage, error) {
	// Return "0x1" as a JSON string
	return json.RawMessage(`"0x1"`), nil
}
