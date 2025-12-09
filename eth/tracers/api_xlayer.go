// X Layer: This file contains the X layer implementation of the tracers API.

package tracers

import (
	"github.com/ethereum/go-ethereum/common"
)

// X Layer: CheckTransactionExists checks if a transaction exists in the canonical chain.
func (api *API) CheckTransactionExists(hash common.Hash) (exists bool, indexDone bool) {
	found, _, _, _, _ := api.backend.GetCanonicalTransaction(hash)
	return found, api.backend.TxIndexDone()
}
