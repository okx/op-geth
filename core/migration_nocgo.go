//go:build !cgo

// Copyright 2024 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.
package core

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/urfave/cli/v2"
)

// ScanDB scans the mdbx database and returns the ignored genesis alloc
func ScanDB(migrationPath string, genesis *Genesis, ignoreAddresses map[common.Address]struct{}) (types.GenesisAlloc, types.GenesisAlloc, error) {
	panic("not implemented")
}

// SetupGenesisBlockWithMigrationData sets up the genesis block with migration data
func SetupGenesisBlockWithMigrationData(chaindb ethdb.Database, triedb *triedb.Database, genesis *Genesis, overrides *ChainOverrides, ctx *cli.Context) (*params.ChainConfig, common.Hash, *params.ConfigCompatError, error) {
	panic("not implemented")
}
