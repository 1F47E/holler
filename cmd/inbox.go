package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/1F47E/holler/identity"
	"github.com/1F47E/holler/message"
	"github.com/spf13/cobra"
)

var (
	inboxLast int
	inboxFrom string
	inboxJSON bool
)

func init() {
	inboxCmd.Flags().IntVarP(&inboxLast, "last", "n", 0, "Show last N messages (0 = all)")
	inboxCmd.Flags().StringVar(&inboxFrom, "from", "", "Filter by sender (alias or onion address)")
	inboxCmd.Flags().BoolVar(&inboxJSON, "json", false, "Raw JSONL output")
	rootCmd.AddCommand(inboxCmd)
}

var inboxCmd = &cobra.Command{
	Use:   "inbox",
	Short: "View received messages",
	RunE: func(cmd *cobra.Command, args []string) error {
		hollerDir, err := identity.HollerDir()
		if err != nil {
			return err
		}

		envelopes, err := loadInbox(hollerDir)
		if err != nil {
			return err
		}

		if len(envelopes) == 0 {
			fmt.Println("Inbox is empty.")
			return nil
		}

		// Resolve --from alias if needed
		var fromOnion string
		if inboxFrom != "" {
			contacts, _ := identity.LoadContacts()
			fromOnion = contacts.Resolve(inboxFrom)
		}

		// Filter by sender
		if fromOnion != "" {
			var filtered []*message.Envelope
			for _, env := range envelopes {
				if env.From == fromOnion {
					filtered = append(filtered, env)
				}
			}
			envelopes = filtered
		}

		// Apply --last
		if inboxLast > 0 && len(envelopes) > inboxLast {
			envelopes = envelopes[len(envelopes)-inboxLast:]
		}

		if len(envelopes) == 0 {
			fmt.Println("No matching messages.")
			return nil
		}

		// Load contacts for alias resolution in display
		contacts, _ := identity.LoadContacts()

		for _, env := range envelopes {
			if inboxJSON {
				data, _ := json.Marshal(env)
				fmt.Println(string(data))
			} else {
				ts := time.Unix(env.Ts, 0).Format("2006-01-02 15:04:05")
				sender := env.From
				if alias, found := contacts.FindByOnion(env.From); found {
					sender = alias
				} else if len(sender) > 16 {
					sender = sender[:16] + "..."
				}
				fmt.Printf("[%s] %s: %s\n", ts, sender, env.Body)
			}
		}
		return nil
	},
}

func loadInbox(hollerDir string) ([]*message.Envelope, error) {
	path := message.InboxPath(hollerDir)
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open inbox: %w", err)
	}
	defer f.Close()

	var envelopes []*message.Envelope
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20) // up to 1MB per line
	for scanner.Scan() {
		var env message.Envelope
		if err := json.Unmarshal(scanner.Bytes(), &env); err != nil {
			continue // skip corrupt lines
		}
		envelopes = append(envelopes, &env)
	}
	return envelopes, scanner.Err()
}
