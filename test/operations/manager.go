package operations

import (
	"context"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

// Public shared
const (
	DefaultL1NetworkURL             = "http://localhost:8545"
	DefaultL1ChainID         uint64 = 1337
	DefaultL1AdminAddress           = "0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534"
	DefaultL1AdminPrivateKey        = "0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2"

	DefaultL2NetworkURL        = "http://localhost:8124"
	DefaultL2SeqURL            = "http://localhost:8123"
	DefaultL2ChainID    uint64 = 195

	DefaultL2MetricsPrometheusURL = "http://127.0.0.1:9092/debug/metrics/prometheus"
	DefaultL2MetricsURL           = "http://127.0.0.1:9092/debug/metrics"

	BridgeAddr = "0x4B24266C13AFEf2bb60e2C69A4C08A482d81e3CA"

	DefaultTimeoutTxToBeMined = 1 * time.Minute

	DefaultL2AdminAddress    = "0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534"
	DefaultL2AdminPrivateKey = "0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2"

	DefaultRichAddress    = "0x14dC79964da2C08b23698B3D3cc7Ca32193d9955"
	DefaultRichPrivateKey = "0x4bbbf85ce3377467afe5d46f804f221813b2bb87f24d81f60f1fcdbf7cbf4356"

	DefaultL2NewAcc1Address    = "0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC"
	DefaultL2NewAcc1PrivateKey = "5de4111afa1a4b94908f83103eb1f1706367c2e68ca870fc3fb9a804cdab365a"
	DefaultL2NewAcc2Address    = "0xAed6892D56AAB5DA8FBcd85b924C3bE63c74Cc29"
	DefaultL2NewAcc2PrivateKey = "bc362a16d3dedd6cdba639eb8fa91b2f6d9f929eb490ca2e5a748ba041c6a131"

	DefaultOkPaySenderAddress    = "0xa0Ee7A142d267C1f36714E4a8F75612F20a79720"
	DefaultOkPaySenderPrivateKey = "2a871d0798f97d79848a013d4936a73bf4cc922c825d33c1cf7073dff6d409c6"
)

var clientRPC *ethclient.Client

func init() {
	var err error
	clientRPC, err = GetRPCClient()
	if err != nil {
		log.Crit("Failed to initialize RPC client", "error", err)
	}
}

// ApplyL1Txs sends the given L1 txs, waits for them to be consolidated and checks the final state.
func ApplyL1Txs(ctx context.Context, txs []*types.Transaction, auth *bind.TransactOpts, client *ethclient.Client) error {
	_, err := applyTxs(ctx, txs, auth, client, true)
	return err
}

// ConfirmationLevel type used to describe the confirmation level of a transaction
type ConfirmationLevel int

// PoolConfirmationLevel indicates that transaction is added into the pool
const PoolConfirmationLevel ConfirmationLevel = 0

// TrustedConfirmationLevel indicates that transaction is  added into the trusted state
const TrustedConfirmationLevel ConfirmationLevel = 1

// VirtualConfirmationLevel indicates that transaction is  added into the virtual state
const VirtualConfirmationLevel ConfirmationLevel = 2

// VerifiedConfirmationLevel indicates that transaction is  added into the verified state
const VerifiedConfirmationLevel ConfirmationLevel = 3

// ApplyL2Txs sends the given L2 txs, waits for them to be consolidated and
// checks the final state.
func ApplyL2Txs(ctx context.Context, txs []*types.Transaction, auth *bind.TransactOpts, client *ethclient.Client, confirmationLevel ConfirmationLevel) ([]*big.Int, error) {
	var err error
	if auth == nil {
		auth, err = GetAuth(DefaultL2AdminPrivateKey, DefaultL2ChainID)
		if err != nil {
			return nil, err
		}
	}

	if client == nil {
		client, err = ethclient.Dial(DefaultL2NetworkURL)
		if err != nil {
			return nil, err
		}
	}
	waitToBeMined := confirmationLevel != PoolConfirmationLevel
	sentTxs, err := applyTxs(ctx, txs, auth, client, waitToBeMined)
	if err != nil {
		return nil, err
	}
	if confirmationLevel == PoolConfirmationLevel {
		return nil, nil
	}

	l2BlockNumbers := make([]*big.Int, 0, len(sentTxs))
	for _, txTemp := range sentTxs {
		// check transaction nonce against transaction reported L2 block number
		receipt, err := client.TransactionReceipt(ctx, txTemp.Hash())
		if err != nil {
			return nil, err
		}

		// get L2 block number
		l2BlockNumbers = append(l2BlockNumbers, receipt.BlockNumber)
		if confirmationLevel == TrustedConfirmationLevel {
			continue
		}

		if confirmationLevel == VirtualConfirmationLevel {
			continue
		}
	}

	return l2BlockNumbers, nil
}

func applyTxs(ctx context.Context, txs []*types.Transaction, auth *bind.TransactOpts, client *ethclient.Client, waitToBeMined bool) ([]*types.Transaction, error) {
	var sentTxs []*types.Transaction

	for i := 0; i < len(txs); i++ {
		signedTx, err := auth.Signer(auth.From, txs[i])
		if err != nil {
			return nil, err
		}
		log.Info("Sending Tx %v Nonce %v", signedTx.Hash(), signedTx.Nonce())
		err = client.SendTransaction(context.Background(), signedTx)
		if err != nil {
			return nil, err
		}

		sentTxs = append(sentTxs, signedTx)
	}
	if !waitToBeMined {
		return nil, nil
	}

	// wait for TX to be mined
	timeout := 180 * time.Second //nolint:gomnd
	for _, tx := range sentTxs {
		log.Info("Waiting Tx %s to be mined", tx.Hash())
		err := WaitTxToBeMined(ctx, client, tx, timeout)
		if err != nil {
			return nil, err
		}
		log.Info("Tx %s mined successfully", tx.Hash())
	}
	nTxs := len(txs)
	if nTxs > 1 {
		log.Info("%d transactions added into the trusted state successfully.", nTxs)
	} else {
		log.Info("transaction added into the trusted state successfully.")
	}

	return sentTxs, nil
}

// GetAuth configures and returns an auth object.
func GetAuth(privateKeyStr string, chainID uint64) (*bind.TransactOpts, error) {
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(privateKeyStr, "0x"))
	if err != nil {
		return nil, err
	}

	return bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(0).SetUint64(chainID))
}

// MustGetAuth GetAuth but panics if err
func MustGetAuth(privateKeyStr string, chainID uint64) *bind.TransactOpts {
	auth, err := GetAuth(privateKeyStr, chainID)
	if err != nil {
		panic(err)
	}
	return auth
}

// GetClient returns an ethereum client to the provided URL
func GetClient(URL string) (*ethclient.Client, error) {
	client, err := ethclient.Dial(URL)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func GetTestChainConfig(chainID uint64) *params.ChainConfig {
	return &params.ChainConfig{
		ChainID:                 big.NewInt(int64(chainID)),
		HomesteadBlock:          big.NewInt(0),
		EIP150Block:             big.NewInt(0),
		EIP155Block:             big.NewInt(0),
		EIP158Block:             big.NewInt(0),
		ByzantiumBlock:          big.NewInt(0),
		ConstantinopleBlock:     big.NewInt(0),
		PetersburgBlock:         big.NewInt(0),
		IstanbulBlock:           big.NewInt(0),
		MuirGlacierBlock:        big.NewInt(0),
		BerlinBlock:             big.NewInt(0),
		LondonBlock:             big.NewInt(0), // EIP-1559 support
		ArrowGlacierBlock:       big.NewInt(0),
		GrayGlacierBlock:        big.NewInt(0),
		MergeNetsplitBlock:      big.NewInt(0),
		ShanghaiTime:            newUint64(0), // Shanghai upgrade
		CancunTime:              newUint64(0), // Cancun upgrade (EIP-4844 blob transactions)
		PragueTime:              newUint64(0), // Prague upgrade
		TerminalTotalDifficulty: big.NewInt(0),
		Ethash:                  new(params.EthashConfig),
		BlobScheduleConfig: &params.BlobScheduleConfig{
			Cancun: params.DefaultCancunBlobConfig,
			Prague: params.DefaultPragueBlobConfig,
		},
	}
}

// newUint64 is a helper to create a pointer to a uint64
func newUint64(val uint64) *uint64 {
	return &val
}

func GetRPCClient() (*ethclient.Client, error) {
	// Create RPC client with reasonable timeouts
	rpcClient, err := ethclient.Dial(DefaultL2NetworkURL)
	if err != nil {
		return nil, err
	}

	return rpcClient, nil
}
