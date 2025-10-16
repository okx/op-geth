// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package rawdb

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"golang.org/x/time/rate"
)

// XlayerAncientProxy is a proxy for legacy header sync from a remote PP RPC endpoint.
// It fetches legacy XLayer block headers during the freezing process for blocks below the migration threshold.
type XlayerAncientProxy struct {
	legacyThreshold uint64
	rpcClient       *xlayerRPCClient
	firstRun        bool       // Track if this is the first freezeRange call
	mu              sync.Mutex // Protect firstRun flag
}

// NewXlayerAncientProxy creates a new proxy for legacy header sync.
// Returns nil if the configuration is invalid.
// The headerRateLimit parameter controls the QPS for legacy header sync from PP RPC.
func NewXlayerAncientProxy(legacyThreshold uint64, ppRPCUrl string, ppRPCTimeout time.Duration, headerRateLimit int) *XlayerAncientProxy {
	// Validate configuration
	if ppRPCUrl == "" {
		log.Warn("XLayer PP RPC URL not configured, proxy disabled")
		return nil
	}

	if legacyThreshold == 0 {
		log.Warn("XLayer legacy threshold is 0, proxy disabled")
		return nil
	}

	// Create RPC client
	client, err := newXLayerRPCClient(ppRPCUrl, ppRPCTimeout, headerRateLimit)
	if err != nil {
		log.Warn("Failed to create XLayer RPC client, proxy disabled", "err", err)
		return nil
	}

	log.Info("Created XLayer ancient proxy for legacy header sync",
		"legacyThreshold", legacyThreshold,
		"ppRPCUrl", ppRPCUrl,
		"ppRPCTimeout", ppRPCTimeout,
		"headerRateLimit", headerRateLimit)

	return &XlayerAncientProxy{
		legacyThreshold: legacyThreshold,
		rpcClient:       client,
		firstRun:        true, // Mark as first run to limit initial sync
	}
}

// Close closes the proxy and releases resources.
func (p *XlayerAncientProxy) Close() {
	if p != nil && p.rpcClient != nil {
		p.rpcClient.Close()
	}
}

// ShouldProxy determines if a block header should be synced via legacy header sync.
// Returns true for blocks in the range (0, legacyThreshold).
func (p *XlayerAncientProxy) ShouldProxy(number uint64) bool {
	if p == nil {
		return false
	}
	// Genesis block (0) is never proxied
	return number > 0 && number < p.legacyThreshold
}

// ShouldBreakAtStartup checks if this is the first run and should break early to avoid blocking startup.
// Returns true if this is the first freezeRange call. After returning true once, it sets firstRun to false.
func (p *XlayerAncientProxy) isFirstRun() bool {
	if p == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.firstRun
}

// setFirstRun
func (p *XlayerAncientProxy) setFirstRun(firstRun bool) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.firstRun = firstRun
}

// FetchLegacyBlockData performs legacy header sync from the PP RPC endpoint.
// Returns (hash, headerRLP, bodyRLP, receiptsRLP, nil) on success.
// For legacy header sync, body and receipts are empty RLP-encoded lists.
func (p *XlayerAncientProxy) FetchLegacyBlockData(number uint64, nfdb ethdb.KeyValueReader) (common.Hash, []byte, []byte, []byte, error) {
	if p == nil || p.rpcClient == nil {
		return common.Hash{}, nil, nil, nil, fmt.Errorf("proxy not initialized")
	}

	// Perform legacy header sync from PP RPC
	hash, headerRLP, err := p.rpcClient.getBlockByNumber(number)
	if err != nil {
		log.Warn("Legacy header sync: failed to fetch block header, will retry later", "number", number, "err", err)
		return common.Hash{}, nil, nil, nil, fmt.Errorf("legacy header sync failed for block %d: %v", number, err)
	}

	// For legacy header sync, body and receipts are empty RLP-encoded lists
	emptyList, _ := rlp.EncodeToBytes([]interface{}{})

	// Log progress periodically (every 1000 blocks) or at DEBUG level for each block
	if number%1000 == 0 || number == p.legacyThreshold-1 {
		progress := float64(number) / float64(p.legacyThreshold) * 100
		log.Info("Legacy header sync: progress update",
			"number", number,
			"hash", hash,
			"total", p.legacyThreshold,
			"progress", fmt.Sprintf("%.2f%%", progress),
			"url", p.rpcClient.url)
	} else {
		log.Debug("Legacy header sync: successfully fetched block header",
			"number", number,
			"hash", hash)
	}

	return hash, headerRLP, emptyList, emptyList, nil
}

// FreezeRangeWithProxy freezes a range of blocks using the XLayer proxy for legacy blocks.
// It handles the entire freeze loop including fetching data from proxy or local DB and writing to freezer.
// Returns the list of block hashes that were successfully frozen.
func (p *XlayerAncientProxy) FreezeRangeWithProxy(op ethdb.AncientWriteOp, nfdb *nofreezedb, number, limit uint64) ([]common.Hash, error) {
	hashes := make([]common.Hash, 0, limit-number+1)

	for ; number <= limit; number++ {
		var hash common.Hash
		var header, body, receipts []byte

		// XLayer: Check if we should perform legacy header sync
		if p.ShouldProxy(number) {
			// Check if we should break early during startup to avoid blocking node initialization
			if p.isFirstRun() {
				p.setFirstRun(false)
				log.Info("Legacy header sync: breaking at startup to allow node to start quickly, will continue in background")
				break
			}

			// Perform legacy header sync from PP RPC
			var proxyErr error
			hash, header, body, receipts, proxyErr = p.FetchLegacyBlockData(number, nfdb)
			if proxyErr != nil {
				break
			}
		} else {
			// Fetch from local database
			hash = ReadCanonicalHash(nfdb, number)
			if hash == (common.Hash{}) {
				return hashes, fmt.Errorf("canonical hash missing, can't freeze block %d", number)
			}
			header = ReadHeaderRLP(nfdb, hash, number)
			if len(header) == 0 {
				return hashes, fmt.Errorf("block header missing, can't freeze block %d", number)
			}
			body = ReadBodyRLP(nfdb, hash, number)
			if len(body) == 0 {
				return hashes, fmt.Errorf("block body missing, can't freeze block %d", number)
			}
			receipts = ReadReceiptsRLP(nfdb, hash, number)
			if len(receipts) == 0 {
				return hashes, fmt.Errorf("block receipts missing, can't freeze block %d", number)
			}
		}

		// Write to the batch.
		if err := op.AppendRaw(ChainFreezerHashTable, number, hash[:]); err != nil {
			return hashes, fmt.Errorf("can't write hash to Freezer: %v", err)
		}
		if err := op.AppendRaw(ChainFreezerHeaderTable, number, header); err != nil {
			return hashes, fmt.Errorf("can't write header to Freezer: %v", err)
		}
		if err := op.AppendRaw(ChainFreezerBodiesTable, number, body); err != nil {
			return hashes, fmt.Errorf("can't write body to Freezer: %v", err)
		}
		if err := op.AppendRaw(ChainFreezerReceiptTable, number, receipts); err != nil {
			return hashes, fmt.Errorf("can't write receipts to Freezer: %v", err)
		}
		hashes = append(hashes, hash)
	}
	return hashes, nil
}

// xlayerRPCClient manages the connection to the legacy PP RPC endpoint for legacy header sync.
type xlayerRPCClient struct {
	client  *rpc.Client
	url     string
	timeout time.Duration
	limiter *rate.Limiter // Rate limiter for legacy header sync QPS control
	mu      sync.RWMutex
}

// newXLayerRPCClient creates a new RPC client for legacy header sync from the PP endpoint.
func newXLayerRPCClient(url string, timeout time.Duration, headerRateLimit int) (*xlayerRPCClient, error) {
	if url == "" {
		return nil, fmt.Errorf("RPC URL is empty")
	}

	if timeout == 0 {
		timeout = 10 * time.Second // Default timeout
	}

	if headerRateLimit <= 0 {
		headerRateLimit = 80 // Default 80 QPS for header sync
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client, err := rpc.DialContext(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PP RPC at %s: %v", url, err)
	}

	// Create rate limiter with burst capacity = 2 * QPS
	// This allows some flexibility for bursty legacy header sync requests while maintaining average rate
	limiter := rate.NewLimiter(rate.Limit(headerRateLimit), headerRateLimit*2)

	log.Debug("Created XLayer RPC client for legacy header sync", "url", url, "timeout", timeout, "headerRateLimit", headerRateLimit)

	return &xlayerRPCClient{
		client:  client,
		url:     url,
		timeout: timeout,
		limiter: limiter,
	}, nil
}

// Close closes the RPC client connection.
func (c *xlayerRPCClient) Close() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.client != nil {
		c.client.Close()
		log.Debug("Closed XLayer RPC client", "url", c.url)
	}
}

// getBlockByNumber performs legacy header sync by fetching a block's hash and header from the PP RPC endpoint.
// Returns (hash, headerRLP, nil) on success, or (zero, nil, error) on failure.
func (c *xlayerRPCClient) getBlockByNumber(number uint64) (common.Hash, []byte, error) {
	if c == nil || c.client == nil {
		return common.Hash{}, nil, fmt.Errorf("RPC client not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	// Wait for rate limiter permission (legacy header sync QPS control)
	if err := c.limiter.Wait(ctx); err != nil {
		return common.Hash{}, nil, fmt.Errorf("legacy header sync rate limiter wait failed: %v", err)
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	// Call eth_getBlockByNumber with full transaction details = false
	var header *types.Header
	blockNum := toBlockNumArg(new(big.Int).SetUint64(number))
	err := c.client.CallContext(ctx, &header, "eth_getBlockByNumber", blockNum, false)
	if err != nil {
		return common.Hash{}, nil, fmt.Errorf("RPC call failed for block %d: %v", number, err)
	}
	if header == nil {
		return common.Hash{}, nil, fmt.Errorf("block %d not found in PP RPC", number)
	}

	// Encode header to RLP
	headerRLP, err := rlp.EncodeToBytes(header)
	if err != nil {
		return common.Hash{}, nil, fmt.Errorf("failed to encode header for block %d: %v", number, err)
	}

	hash := header.Hash()
	// Dump header info for debugging
	log.Trace("Legacy header sync: fetched block header details", "number", number, "hash", hash, "parentHash", header.ParentHash, "nonce", header.Nonce, "logsBloom", header.Bloom, "transactionsRoot", header.TxHash, "stateRoot", header.Root, "receiptsRoot", header.ReceiptHash, "number", header.Number, "gasLimit", header.GasLimit, "gasUsed", header.GasUsed, "timestamp", header.Time, "extraData", header.Extra, "mixHash", header.MixDigest, "nonce", header.Nonce, "difficulty", header.Difficulty, "totalDifficulty", header.Difficulty)

	return hash, headerRLP, nil
}

// toBlockNumArg converts a big.Int block number to the RPC argument format.
func toBlockNumArg(number *big.Int) string {
	if number == nil {
		return "latest"
	}
	return fmt.Sprintf("0x%x", number)
}
