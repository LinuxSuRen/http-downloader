package installer

func (o *Installer) runCommandList(cmds []CmdWithArgs) (err error) {
	for i := range cmds {
		cmd := cmds[i]
		if err = o.Execer.RunCommand(cmd.Cmd, cmd.Args...); err != nil {
			return
		}
	}
	return
}
