// Copyright 2021 The go-ethereum Authors
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

package tracers

import (
	"github.com/ethereum/go-ethereum/common"
)

// X Layer: CheckTransactionExists checks if a transaction exists in the canonical chain.
func (api *API) CheckTransactionExists(hash common.Hash) (exists bool, indexDone bool) {
	found, _, _, _, _ := api.backend.GetCanonicalTransaction(hash)
	return found, api.backend.TxIndexDone()
}
