package main

import "github.com/ethereum/go-ethereum/cmd/utils"

func init() {
	nodeFlags = append(nodeFlags, utils.P2PETH69CompatFlag)
}
