package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/1F47E/holler/daemon"
	"github.com/1F47E/holler/identity"
	"github.com/1F47E/holler/message"
	"github.com/1F47E/holler/node"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(runDaemonCmd)
}

var runDaemonCmd = &cobra.Command{
	Use:    "_run-daemon",
	Short:  "Internal: run the daemon process (do not call directly)",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		hollerDir, err := identity.HollerDir()
		if err != nil {
			return err
		}

		if err := node.CheckTorAvailable(); err != nil {
			return err
		}

		onionKey, err := node.LoadOrCreateOnionKey(hollerDir)
		if err != nil {
			return err
		}
		onionAddr := identity.OnionAddrFromKey(onionKey)
		kp := identity.OnionKeyPairFromBine(onionKey)

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		// Message handler — write to inbox + run hook
		msgHandler := func(env *message.Envelope) {
			data, err := json.Marshal(env)
			if err != nil {
				return
			}
			message.AppendToInbox(hollerDir, data)
			daemon.RunReceiveHook(hollerDir, env)
		}

		tn, err := node.ListenTor(onionKey, onionAddr)
		if err != nil {
			return err
		}
		defer tn.Close()

		fmt.Fprintf(os.Stderr, "[%s] daemon started: %s.onion:9000\n",
			time.Now().Format("2006-01-02 15:04:05"), onionAddr)

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

		// Start outbox retry
		go retryOutboxLoop(ctx, hollerDir)

		<-ctx.Done()
		fmt.Fprintf(os.Stderr, "[%s] daemon shutting down\n",
			time.Now().Format("2006-01-02 15:04:05"))
		daemon.RemovePid(hollerDir)
		return nil
	},
}
