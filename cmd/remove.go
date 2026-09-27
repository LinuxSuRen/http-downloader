package cmd

import (
	"context"
	sysos "os"
	"path/filepath"
	"strings"

	fakeruntime "github.com/linuxsuren/go-fake-runtime"
	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/spf13/cobra"
)

func newRemoveCmd(ctx context.Context) (cmd *cobra.Command) {
	opt := &removeOption{
		execer: &fakeruntime.DefaultExecer{},
	}
	cmd = &cobra.Command{
		Use:     "remove",
		Aliases: []string{"rm", "uninstall", "un"},
		Short:   "Remove the tools which are installed by hd",
		Example: "hd remove jcli",
		Args:    cobra.MinimumNArgs(1),
		RunE:    opt.runE,
		GroupID: coreGroup.ID,
	}
	cmd.Flags().BoolVarP(&opt.keepRecord, "keep-record", "", false,
		"Only remove the record, keep the binary files")
	return
}

type removeOption struct {
	keepRecord bool
	execer     fakeruntime.Execer
}

func (o *removeOption) runE(cmd *cobra.Command, args []string) (err error) {
	for _, name := range args {
		tool, removeErr := installer.RemoveInstalledPackage(name)
		if removeErr != nil {
			cmd.Printf("cannot remove %s, error: %v\n", name, removeErr)
			err = removeErr
			continue
		}

		if o.keepRecord {
			cmd.Printf("removed record of %s\n", name)
			continue
		}

		if tool.Native {
			cmd.Printf("please remove the native package [%s] by yourself\n", name)
			continue
		}

		for _, binary := range tool.Binaries {
			if binary == "" {
				continue
			}
			if rmErr := o.removeBinary(binary); rmErr != nil {
				cmd.Printf("cannot remove binary %s, error: %v\n", binary, rmErr)
				if err == nil {
					err = rmErr
				}
			} else {
				cmd.Printf("removed %s\n", binary)
			}
		}
	}
	return
}

func (o *removeOption) removeBinary(binary string) (err error) {
	if _, statErr := sysos.Stat(binary); statErr != nil {
		if sysos.IsNotExist(statErr) {
			// the binary is already gone
			return nil
		}
		return statErr
	}

	if err = sysos.Remove(binary); err != nil && strings.Contains(err.Error(), "permission denied") {
		_ = err
		if sudoErr := o.execer.RunCommandWithSudo("rm", "-f", filepath.Clean(binary)); sudoErr == nil {
			err = nil
		}
	}
	return err
}
