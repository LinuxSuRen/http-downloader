package cmd

import (
	"context"
	"net/http"

	"github.com/spf13/cobra"
)

func getOrDefault(key, def string, data map[string]string) (result string) {
	var ok bool
	if result, ok = data[key]; !ok {
		result = def
	}
	return
}

type contextRoundTripper string

func getRoundTripper(ctx context.Context) (tripper http.RoundTripper) {
	if ctx == nil {
		return
	}
	roundTripper := ctx.Value(contextRoundTripper("roundTripper"))

	switch v := roundTripper.(type) {
	case *http.Transport:
		tripper = v
	}
	return
}

// CompletionFunc is the function for command completion
type CompletionFunc func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)

// ArrayCompletion return a completion  which base on an array
func ArrayCompletion(array ...string) CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return array, cobra.ShellCompDirectiveNoFileComp
	}
}
