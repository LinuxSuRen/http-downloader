package main

import (
	installer "github.com/linuxsuren/http-downloader/pkg"
)

func main() {
	targetURL := "https://github.com/LinuxSuRen/http-downloader/releases/download/v0.0.55/hd-linux-amd64.tar.gz"
	if err := installer.DownloadWithContinue(targetURL,
		"test.tar.gz", 0, -1, 0, true); err != nil {
		panic(err)
	}
}
