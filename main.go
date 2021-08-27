package main

import (
	"context"
	"github.com/linuxsuren/http-downloader/cmd"
	"os"
)

func main() {
	if err := cmd.NewRoot(context.TODO()).Execute(); err != nil {
		os.Exit(1)
	}
}
