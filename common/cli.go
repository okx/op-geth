package common

import (
	"strings"
)

func CliString2Array(input string) []string {
	l := strings.Split(input, ",")
	res := make([]string, 0, len(l))
	for _, r := range l {
		if r = strings.TrimSpace(r); r != "" {
			res = append(res, r)
		}
	}
	return res
}
