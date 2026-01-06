package apollo

import (
	"flag"
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"
)

const (
	NamespaceSplits = 2
)

// createMockContext creates a mock CLI context for testing
func createMockContext(flags []cli.Flag) *cli.Context {
	set := flag.NewFlagSet("", flag.ContinueOnError)
	for _, f := range flags {
		if f != nil {
			f.Apply(set)
		}
	}

	context := cli.NewContext(nil, set, nil)
	return context
}

func getNamespacePrefix(namespace string) (string, error) {
	items := strings.Split(namespace, "-")
	if len(items) < NamespaceSplits {
		return "", fmt.Errorf("invalid namespace: %s, no separator \"-\" present, please configure apollo namespace in the correct format \"prefix-item\"", namespace)
	}
	return items[0], nil
}

func getNamespaceSuffix(namespace string) (string, error) {
	items := strings.Split(namespace, "-")
	if len(items) < NamespaceSplits {
		return "", fmt.Errorf("invalid namespace: %s, no separator \"-\" present, please configure apollo namespace in the correct format \"item-suffix\"", namespace)
	}
	return items[len(items)-1], nil
}

func SanitizeFlags(flags []cli.Flag) []cli.Flag {
	seen := make(map[string]struct{})
	var result []cli.Flag

	for _, flag := range flags {
		if flag == nil {
			continue // skip nil flags
		}

		// Use flag name as key to detect duplicates
		flagName := flag.Names()[0]
		if _, ok := seen[flagName]; ok {
			continue // skip duplicate flags
		}

		seen[flagName] = struct{}{}
		result = append(result, flag)
	}

	return result
}
