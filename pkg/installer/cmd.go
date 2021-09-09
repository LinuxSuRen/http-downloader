package installer

import "github.com/linuxsuren/http-downloader/pkg/exec"

func runCommandList(cmds []CmdWithArgs) (err error) {
	for i := range cmds {
		cmd := cmds[i]
		if err = exec.RunCommand(cmd.Cmd, cmd.Args...); err != nil {
			return
		}
	}
	return
}
