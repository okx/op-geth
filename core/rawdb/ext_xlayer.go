// Copyright 2025 The xlayer-geth Authors
// This file is part of xlayer-geth.
//
// xlayer-geth is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// xlayer-geth is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with xlayer-geth. If not, see <http://www.gnu.org/licenses/>.

package rawdb

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
)

// ReadChainConfigRaw reads the raw chain config JSON data from database
// This is exposed for xlayer-geth to decode only the fields it needs
func ReadChainConfigRaw(db ethdb.KeyValueReader, hash common.Hash) ([]byte, error) {
	data, err := db.Get(configKey(hash))
	return data, err
}

