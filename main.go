package main

import (
	"github.com/spf13/cobra"
)

func main()  {
	cmd := &cobra.Command{
		Use: "github-go",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println("hello")
		},
	}
	if err := cmd.Execute(); err != nil {
		panic(err)
	}
}
