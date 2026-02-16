package cmd

import (
	"github.com/1F47E/holler/identity"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "holler",
	Short: "P2P encrypted messenger for AI agents",
	Long:  "holler — peer-to-peer encrypted messaging over libp2p. No servers, no registration. Identity is a keypair.",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&identity.DirOverride, "dir", "", "data directory (default ~/.holler)")
}

func Execute() error {
	return rootCmd.Execute()
}
