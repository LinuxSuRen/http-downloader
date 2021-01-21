package main

import (
	"github.com/linuxsuren/http-downloader/cmd"
	"os"
)

func main() {
	if err := cmd.NewRoot().Execute(); err != nil {
		os.Exit(1)
	}
}
