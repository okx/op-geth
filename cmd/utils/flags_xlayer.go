package utils

import (
	"time"

	"github.com/ethereum/go-ethereum/internal/apollo"
	"github.com/ethereum/go-ethereum/internal/flags"
	"github.com/urfave/cli/v2"
)

var (
	// Apollo configuration flags
	ApolloEnabledFlag = &cli.BoolFlag{
		Name:     "apollo.enabled",
		Usage:    "Enable Apollo configuration client",
		Category: flags.ApolloCategory,
	}
	ApolloEndpointFlag = &cli.StringFlag{
		Name:     "apollo.endpoint",
		Usage:    "Apollo endpoint",
		Category: flags.ApolloCategory,
	}
	ApolloAppIDFlag = &cli.StringFlag{
		Name:     "apollo.app-id",
		Usage:    "Apollo application ID",
		Category: flags.ApolloCategory,
	}
	ApolloClusterFlag = &cli.StringFlag{
		Name:     "apollo.cluster",
		Usage:    "Apollo cluster name",
		Value:    "default",
		Category: flags.ApolloCategory,
	}
	ApolloNamespaceFlag = &cli.StringFlag{
		Name:     "apollo.namespace",
		Usage:    "Apollo namespace name",
		Value:    "application",
		Category: flags.ApolloCategory,
	}
	ApolloSecretFlag = &cli.StringFlag{
		Name:     "apollo.secret",
		Usage:    "Apollo secret key for authentication",
		Category: flags.ApolloCategory,
	}
	ApolloSyncTimeoutFlagName = &cli.DurationFlag{
		Name:     "apollo.sync-timeout",
		Usage:    "Apollo API request timeout",
		Value:    10 * time.Second,
		Category: flags.ApolloCategory,
	}
)

// SetApolloConfig applies Apollo-related command line flags to the config.
func SetApolloConfig(ctx *cli.Context, cfg *apollo.Config) {
	if ctx.IsSet(ApolloEnabledFlag.Name) {
		cfg.Enabled = ctx.Bool(ApolloEnabledFlag.Name)
	}
	if ctx.IsSet(ApolloEndpointFlag.Name) {
		cfg.Endpoint = ctx.String(ApolloEndpointFlag.Name)
	}
	if ctx.IsSet(ApolloAppIDFlag.Name) {
		cfg.AppID = ctx.String(ApolloAppIDFlag.Name)
	}
	cfg.Cluster = ctx.String(ApolloClusterFlag.Name)
	cfg.Namespace = ctx.String(ApolloNamespaceFlag.Name)
	if ctx.IsSet(ApolloSecretFlag.Name) {
		cfg.Secret = ctx.String(ApolloSecretFlag.Name)
	}
	if ctx.IsSet(ApolloSyncTimeoutFlagName.Name) {
		cfg.SyncTimeout = ctx.Duration(ApolloSyncTimeoutFlagName.Name)
	}
}
