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

package gethmain

import (
	"github.com/ethereum/go-ethereum/node"
	"github.com/urfave/cli/v2"
)

// Exposed variables for xlayer-geth
var (
	App          = app          // CLI application
	NodeFlags    = nodeFlags    // Node configuration flags
	RPCFlags     = rpcFlags     // RPC configuration flags
	MetricsFlags = metricsFlags // Metrics configuration flags
	ConsoleFlags = consoleFlags // Console configuration flags
)

// Prepare exposes the prepare function for xlayer-geth
func Prepare(ctx *cli.Context) {
	prepare(ctx)
}

// MakeFullNode exposes the makeFullNode function for xlayer-geth
func MakeFullNode(ctx *cli.Context) *node.Node {
	return makeFullNode(ctx)
}

// StartNode exposes the startNode function for xlayer-geth
func StartNode(ctx *cli.Context, stack *node.Node, isConsole bool) {
	startNode(ctx, stack, isConsole)
}
