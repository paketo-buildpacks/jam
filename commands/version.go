package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var jamVersion string

func version() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "version of jam",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("jam %s\n", jamVersion)
		},
	}

	return cmd
}

func init() {
	rootCmd.AddCommand(version())
}
