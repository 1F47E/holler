package cmd

import (
	"fmt"

	"github.com/1F47E/holler/identity"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(idCmd)
}

var idCmd = &cobra.Command{
	Use:   "id",
	Short: "Print your PeerID",
	RunE: func(cmd *cobra.Command, args []string) error {
		key, err := identity.LoadOrFail()
		if err != nil {
			return err
		}
		pid, err := identity.PeerIDFromKey(key)
		if err != nil {
			return err
		}
		fmt.Println(pid.String())
		return nil
	},
}
