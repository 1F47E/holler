package cmd

import (
	"fmt"

	"github.com/1F47E/holler/identity"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate keypair and print PeerID",
	RunE: func(cmd *cobra.Command, args []string) error {
		if identity.KeyExists() {
			key, err := identity.LoadOrFail()
			if err != nil {
				return err
			}
			pid, err := identity.PeerIDFromKey(key)
			if err != nil {
				return err
			}
			fmt.Printf("Identity already exists: %s\n", pid.String())
			return nil
		}

		key, err := identity.GenerateKey()
		if err != nil {
			return err
		}

		path, err := identity.KeyPath()
		if err != nil {
			return err
		}

		if err := identity.SaveKey(path, key); err != nil {
			return err
		}

		pid, err := identity.PeerIDFromKey(key)
		if err != nil {
			return err
		}

		fmt.Printf("Identity created: %s\n", pid.String())
		fmt.Printf("Key saved to: %s\n", path)
		return nil
	},
}
