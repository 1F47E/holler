package cmd

import (
	"fmt"

	"github.com/1F47E/holler/identity"
	"github.com/1F47E/holler/node"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(idCmd)
}

var idCmd = &cobra.Command{
	Use:   "id",
	Short: "Print your onion address",
	RunE: func(cmd *cobra.Command, args []string) error {
		hollerDir, err := identity.HollerDir()
		if err != nil {
			return err
		}
		onionKey, err := node.LoadOrCreateOnionKey(hollerDir)
		if err != nil {
			return fmt.Errorf("no identity — run 'holler init' first: %w", err)
		}
		fmt.Printf("%s.onion\n", identity.OnionAddrFromKey(onionKey))
		return nil
	},
}
