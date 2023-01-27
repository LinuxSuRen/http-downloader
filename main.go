package main

import (
	"context"
	"os"

	"github.com/linuxsuren/http-downloader/cmd"
)

func main() {
	ctx := context.Background()
	if err := cmd.NewRoot(ctx).ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
