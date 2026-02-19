package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/1F47E/holler/identity"
	"github.com/1F47E/holler/message"
	"github.com/1F47E/holler/node"
	"github.com/spf13/cobra"
)

var listenDaemon bool

func init() {
	listenCmd.Flags().BoolVar(&listenDaemon, "daemon", false, "Write to inbox.jsonl instead of stdout")
	rootCmd.AddCommand(listenCmd)
}

var listenCmd = &cobra.Command{
	Use:   "listen",
	Short: "Listen for incoming messages",
	RunE: func(cmd *cobra.Command, args []string) error {
		if listenDaemon {
			fmt.Fprintf(os.Stderr, "Note: use 'holler daemon start' for background mode\n")
		}

		hollerDir, err := identity.HollerDir()
		if err != nil {
			return err
		}

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()

		if err := node.CheckTorAvailable(); err != nil {
			return err
		}
		onionKey, err := node.LoadOrCreateOnionKey(hollerDir)
		if err != nil {
			return err
		}
		onionAddr := identity.OnionAddrFromKey(onionKey)
		kp := identity.OnionKeyPairFromBine(onionKey)

		// Message handler
		msgHandler := func(env *message.Envelope) {
			data, err := json.Marshal(env)
			if err != nil {
				return
			}
			message.AppendToInbox(hollerDir, data)
			if !listenDaemon {
				fmt.Println(string(data))
			}
		}

		tn, err := node.ListenTor(onionKey, onionAddr)
		if err != nil {
			return err
		}
		defer tn.Close()

		fmt.Fprintf(os.Stderr, "Listening as %s.onion:9000\n", onionAddr)
		if listenDaemon {
			fmt.Fprintf(os.Stderr, "Daemon mode: writing to %s\n", message.InboxPath(hollerDir))
		}

		// Start message handler
		go node.HandleTorConnections(ctx, tn, kp, msgHandler)

		// Start homepage
		profile := node.LoadProfile(hollerDir)
		go node.StartHomepage(ctx, tn.HTTPListener(), node.HomepageData{
			Name:      profile.Name,
			Bio:       profile.Bio,
			OnionAddr: onionAddr,
			Version:   Version,
		})
		fmt.Fprintf(os.Stderr, "Homepage: http://%s.onion\n", onionAddr)

		// Start outbox retry
		go retryOutboxLoop(ctx, hollerDir)

		<-ctx.Done()
		fmt.Fprintf(os.Stderr, "\nShutting down...\n")
		return nil
	},
}

func retryOutboxLoop(ctx context.Context, hollerDir string) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			processOutbox(ctx, hollerDir)
		}
	}
}

func processOutbox(ctx context.Context, hollerDir string) {
	entries, err := message.LoadOutbox(hollerDir)
	if err != nil || len(entries) == 0 {
		return
	}

	now := time.Now().Unix()
	var remaining []message.OutboxEntry
	delivered := 0

	for _, entry := range entries {
		if entry.NextRetry > now {
			remaining = append(remaining, entry)
			continue
		}
		if entry.Attempts >= message.MaxRetries {
			fmt.Fprintf(os.Stderr, "outbox: giving up on message %s after %d attempts\n", entry.Envelope.ID, entry.Attempts)
			continue
		}

		toOnion := entry.Envelope.To
		if len(toOnion) != 56 {
			entry.Attempts++
			entry.NextRetry = time.Now().Add(message.NextBackoff(entry.Attempts)).Unix()
			remaining = append(remaining, entry)
			continue
		}

		if err := deliverOutboxEntry(ctx, toOnion, entry.Envelope); err != nil {
			entry.Attempts++
			entry.NextRetry = time.Now().Add(message.NextBackoff(entry.Attempts)).Unix()
			remaining = append(remaining, entry)
			continue
		}

		delivered++
		fmt.Fprintf(os.Stderr, "outbox: delivered message %s to %s.onion\n", entry.Envelope.ID, toOnion[:16])
	}

	if delivered > 0 {
		fmt.Fprintf(os.Stderr, "outbox: delivered %d pending message(s)\n", delivered)
	}
	if err := message.WriteOutbox(hollerDir, remaining); err != nil {
		fmt.Fprintf(os.Stderr, "outbox: failed to write: %v\n", err)
	}
}

func deliverOutboxEntry(ctx context.Context, toOnion string, env *message.Envelope) error {
	connectCtx, connectCancel := context.WithTimeout(ctx, 120*time.Second)
	defer connectCancel()

	conn, err := node.DialTor(connectCtx, toOnion, 9000)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := node.SendTor(conn, env); err != nil {
		return err
	}
	// Wait for ack (best effort)
	node.RecvTor(conn)
	return nil
}
