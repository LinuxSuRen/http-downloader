package main

import (
	"fmt"
	"github.com/linuxsuren/http-downloader/pkg/installer"
	"os"
	"os/exec"
	"runtime"
)

func main() {
	is := installer.Installer{
		Provider: "github",
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		Fetch:    true,
	}
	if err := is.CheckDepAndInstall(map[string]string{
		"hd": "linuxsuren/http-downloader",
	}); err != nil {
		panic(err)
	}

	if binary, err := exec.LookPath("hd"); err != nil {
		fmt.Println("cannot found the command", err)
		panic(err)
	} else {
		cmd := exec.Command(binary, "version")
		cmd.Stderr = os.Stdout // TODO this might be a bug a http-downloader
		if data, err := cmd.Output(); err != nil {
			fmt.Println("failed to run command", err)
		} else {
			fmt.Print(string(data))
		}
	}
}
