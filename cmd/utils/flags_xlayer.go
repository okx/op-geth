package utils

import (
	libcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/urfave/cli/v2"
)

var (
	// OkPay
	OkPayPriorityEnableFlag = cli.BoolFlag{
		Name:  "okpay.enable-flag",
		Usage: "OkPay",
	}
	OkPaySenderAccountsList = cli.StringFlag{
		Name:  "okpay.sender-accounts-list",
		Usage: "List of OkPay sender accounts",
	}
	OkPayBlockPriorityTxsLimit = cli.Uint64Flag{
		Name:  "okpay.block-priority-txs-limit",
		Usage: "Max number of OkPay txs that we will prioritize per block",
	}
)

func setOkPayXLayer(ctx *cli.Context, cfg *ethconfig.Config) {
	if ctx.IsSet(OkPayPriorityEnableFlag.Name) {
		cfg.XLayer.OkPay.Enable = ctx.Bool(OkPayPriorityEnableFlag.Name)
	}
	if !cfg.XLayer.OkPay.Enable {
		return
	}
	if ctx.IsSet(OkPayBlockPriorityTxsLimit.Name) {
		cfg.XLayer.OkPay.OkPayBlockPriorityTxsLimit = ctx.Uint64(OkPayBlockPriorityTxsLimit.Name)
	}
	if ctx.IsSet(OkPaySenderAccountsList.Name) {
		addrHexes := libcommon.CliString2Array(ctx.String(OkPaySenderAccountsList.Name))
		cfg.XLayer.OkPay.OkPaySenderAccountsList = *libcommon.NewOrderedListOfAddresses(len(addrHexes))
		for _, senderHex := range addrHexes {
			cfg.XLayer.OkPay.OkPaySenderAccountsList.Add(libcommon.HexToAddress(senderHex))
		}
		cfg.XLayer.OkPay.OkPaySenderAccountsList.Sort()
	}
}

// SetOkPayXLayer is a public wrapper function to internally call setOkPayXLayer
func SetOkPayXLayer(ctx *cli.Context, cfg *ethconfig.Config) {
	setOkPayXLayer(ctx, cfg)
}
