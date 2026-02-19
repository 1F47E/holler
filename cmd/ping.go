package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/1F47E/holler/identity"
	"github.com/1F47E/holler/message"
	"github.com/1F47E/holler/node"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(pingCmd)
}

var pingCmd = &cobra.Command{
	Use:   "ping <alias|onion-addr>",
	Short: "Check if a peer is online",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target := args[0]

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()

		if err := node.CheckTorSOCKS(); err != nil {
			return err
		}

		hollerDir, err := identity.HollerDir()
		if err != nil {
			return err
		}
		onionKey, err := node.LoadOrCreateOnionKey(hollerDir)
		if err != nil {
			return err
		}
		myOnion := identity.OnionAddrFromKey(onionKey)
		kp := identity.OnionKeyPairFromBine(onionKey)

		// Resolve target
		contacts, err := identity.LoadContacts()
		if err != nil {
			return err
		}
		toOnion := contacts.Resolve(target)
		if !identity.ValidOnionAddr(toOnion) {
			return fmt.Errorf("cannot resolve %q to a contact — add it with: holler contacts add %s <onion-address>", target, target)
		}

		env := message.NewEnvelope(myOnion, toOnion, "ping", "")
		if err := env.Sign(kp); err != nil {
			return fmt.Errorf("sign message: %w", err)
		}

		fmt.Fprintf(os.Stderr, "Connecting to %s.onion...\n", toOnion[:16])
		connectCtx, connectCancel := context.WithTimeout(ctx, 120*time.Second)
		defer connectCancel()

		conn, err := node.DialTor(connectCtx, toOnion, 9000)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Peer %s.onion unreachable: %v\n", toOnion[:16], err)
			return nil
		}
		defer conn.Close()

		start := time.Now()
		if err := node.SendTor(conn, env); err != nil {
			fmt.Fprintf(os.Stderr, "Send failed: %v\n", err)
			return nil
		}

		ack, err := node.RecvTor(conn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "No ack: %v\n", err)
			return nil
		}
		rtt := time.Since(start)

		if ack.Type == "ack" {
			if valid, verr := ack.Verify(); verr != nil || !valid {
				fmt.Fprintf(os.Stderr, "Ack signature invalid\n")
				return nil
			}
			fmt.Printf("pong from %s.onion: rtt=%s\n", toOnion[:16], rtt.Round(time.Millisecond))
		} else {
			fmt.Fprintf(os.Stderr, "Unexpected response type: %s\n", ack.Type)
		}
		return nil
	},
}
