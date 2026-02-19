package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/1F47E/holler/identity"
	"github.com/1F47E/holler/node"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate Tor identity and print onion address",
	RunE: func(cmd *cobra.Command, args []string) error {
		hollerDir, err := identity.HollerDir()
		if err != nil {
			return err
		}

		onionKey, err := node.LoadOrCreateOnionKey(hollerDir)
		if err != nil {
			return err
		}
		onionAddr := identity.OnionAddrFromKey(onionKey)

		// Create hooks directory
		hooksDir := filepath.Join(hollerDir, "hooks")
		if err := os.MkdirAll(hooksDir, 0755); err != nil {
			return fmt.Errorf("create hooks dir: %w", err)
		}

		// Write on-receive.sample if it doesn't exist
		samplePath := filepath.Join(hooksDir, "on-receive.sample")
		if _, err := os.Stat(samplePath); os.IsNotExist(err) {
			sample := `#!/bin/sh
# on-receive hook — called by the daemon for each incoming message.
# Rename to "on-receive" and chmod +x to enable.
#
# Stdin: full JSON envelope
# Environment variables:
#   HOLLER_MSG_ID    — message UUID
#   HOLLER_MSG_FROM  — sender onion address (56 chars)
#   HOLLER_MSG_TYPE  — message type (message, ping, task-proposal, etc.)
#   HOLLER_MSG_BODY  — message body (first 256 chars)
#   HOLLER_MSG_TS    — unix timestamp
#
# Example: forward to a notification service
# curl -s -X POST "https://ntfy.sh/my-holler" -d "$HOLLER_MSG_BODY"

echo "Received message $HOLLER_MSG_ID from $HOLLER_MSG_FROM"
`
			os.WriteFile(samplePath, []byte(sample), 0644)
		}

		fmt.Printf("Identity: %s.onion\n", onionAddr)
		fmt.Printf("Key:      %s/tor_key\n", hollerDir)
		fmt.Printf("Hooks:    %s/\n", hooksDir)
		return nil
	},
}
