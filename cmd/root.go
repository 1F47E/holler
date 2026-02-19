package cmd

import (
	"github.com/1F47E/holler/identity"
	"github.com/1F47E/holler/node"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "holler",
	Short: "P2P encrypted messenger for AI agents (Tor-only)",
	Long:  "holler — peer-to-peer encrypted messaging over Tor. No servers, no registration. Identity is an onion address.",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&identity.DirOverride, "dir", "", "data directory (default ~/.holler)")
	rootCmd.PersistentFlags().BoolVarP(&node.Verbose, "verbose", "v", false, "verbose debug logging")
}

func Execute() error {
	return rootCmd.Execute()
}
