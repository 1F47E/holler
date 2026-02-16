package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/1F47E/holler/identity"
	"github.com/1F47E/holler/message"
	"github.com/spf13/cobra"
)

func init() {
	outboxCmd.AddCommand(outboxClearCmd)
	rootCmd.AddCommand(outboxCmd)
}

var outboxCmd = &cobra.Command{
	Use:   "outbox",
	Short: "Inspect pending messages in outbox",
	RunE: func(cmd *cobra.Command, args []string) error {
		hollerDir, err := identity.HollerDir()
		if err != nil {
			return err
		}
		entries, err := message.LoadOutbox(hollerDir)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			fmt.Println("Outbox is empty.")
			return nil
		}
		fmt.Fprintf(os.Stderr, "%d pending message(s):\n", len(entries))
		for _, entry := range entries {
			info := map[string]interface{}{
				"id":         entry.Envelope.ID,
				"to":         entry.Envelope.To,
				"body":       entry.Envelope.Body,
				"attempts":   entry.Attempts,
				"next_retry": time.Unix(entry.NextRetry, 0).Format(time.RFC3339),
			}
			data, _ := json.Marshal(info)
			fmt.Println(string(data))
		}
		return nil
	},
}

var outboxClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all pending messages from outbox",
	RunE: func(cmd *cobra.Command, args []string) error {
		hollerDir, err := identity.HollerDir()
		if err != nil {
			return err
		}
		path := message.OutboxPath(hollerDir)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("clear outbox: %w", err)
		}
		fmt.Println("Outbox cleared.")
		return nil
	},
}
