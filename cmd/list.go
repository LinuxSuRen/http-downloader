package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/linuxsuren/http-downloader/pkg/installer"
	"github.com/spf13/cobra"
)

func newListCmd(ctx context.Context) (cmd *cobra.Command) {
	opt := &listOption{}
	cmd = &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List the tools which are installed by hd",
		Example: "hd list\nhd list -o json",
		Args:    cobra.NoArgs,
		RunE:    opt.runE,
		GroupID: coreGroup.ID,
	}
	cmd.Flags().StringVarP(&opt.Format, "output", "o", "table", "The output format: table, json")
	return
}

type listOption struct {
	Format string
}

func (o *listOption) runE(cmd *cobra.Command, args []string) (err error) {
	tools, err := installer.LoadInstalledPackages()
	if err != nil {
		return
	}

	switch o.Format {
	case "json":
		var data []byte
		if data, err = json.MarshalIndent(tools, "", "  "); err == nil {
			cmd.Println(string(data))
		}
	case "table", "":
		if len(tools) == 0 {
			cmd.Println("no installed tools, install one via: hd install <name>")
			return
		}
		writer := tabwriter.NewWriter(cmd.OutOrStdout(), 10, 4, 3, ' ', 0)
		_, _ = fmt.Fprintln(writer, "NAME\tVERSION\tSOURCE\tINSTALLED AT\tDIRECTORY")
		for i := range tools {
			tool := tools[i]
			source := tool.Org + "/" + tool.Repo
			if tool.Native {
				source = "native"
			} else if tool.FromSource {
				source += " (source)"
			}
			_, _ = fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n",
				tool.Name, tool.Version, source, tool.InstalledAt, tool.TargetDirectory)
		}
		_ = writer.Flush()
	default:
		err = fmt.Errorf("unsupported output format: %s", o.Format)
	}
	return
}
