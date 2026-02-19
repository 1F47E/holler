package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/1F47E/holler/daemon"
	"github.com/1F47E/holler/identity"
	"github.com/1F47E/holler/message"
	"github.com/1F47E/holler/node"
	"github.com/spf13/cobra"
)

var (
	sendStdin   bool
	sendType    string
	sendReplyTo string
	sendThread  string
	sendMeta    []string
)

func init() {
	sendCmd.Flags().BoolVar(&sendStdin, "stdin", false, "Read message body from stdin")
	sendCmd.Flags().StringVar(&sendType, "type", "message", "Message type (message, task-proposal, task-result, capability-query, status-update)")
	sendCmd.Flags().StringVar(&sendReplyTo, "reply-to", "", "Message ID this is replying to (for threading)")
	sendCmd.Flags().StringVar(&sendThread, "thread", "", "Thread ID to continue a conversation")
	sendCmd.Flags().StringSliceVar(&sendMeta, "meta", nil, "Metadata key=value pairs (can be repeated)")
	rootCmd.AddCommand(sendCmd)
}

var sendCmd = &cobra.Command{
	Use:   "send <alias|onion-addr> [message]",
	Short: "Send a message to a peer",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target := args[0]
		var body string

		if sendStdin {
			scanner := bufio.NewScanner(os.Stdin)
			var lines []string
			for scanner.Scan() {
				lines = append(lines, scanner.Text())
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("read stdin: %w", err)
			}
			body = strings.Join(lines, "\n")
		} else {
			if len(args) < 2 {
				return fmt.Errorf("provide a message or use --stdin")
			}
			body = strings.Join(args[1:], " ")
		}

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

		// Resolve target via contacts
		contacts, err := identity.LoadContacts()
		if err != nil {
			return err
		}
		toOnion := contacts.Resolve(target)

		if !identity.ValidOnionAddr(toOnion) {
			return fmt.Errorf("cannot resolve %q to a contact — add it with: holler contacts add %s <onion-address>", target, target)
		}

		// Build envelope
		env := message.NewEnvelope(myOnion, toOnion, sendType, body)
		env.ReplyTo = sendReplyTo
		switch {
		case sendThread != "":
			env.ThreadID = sendThread
		case sendReplyTo != "":
			env.ThreadID = sendReplyTo
		default:
			env.ThreadID = env.ID
		}
		if len(sendMeta) > 0 {
			env.Meta = make(map[string]string)
			for _, kv := range sendMeta {
				if k, v, ok := strings.Cut(kv, "="); ok {
					env.Meta[k] = v
				}
			}
		}
		if err := env.Sign(kp); err != nil {
			return fmt.Errorf("sign message: %w", err)
		}

		// Dial and send
		fmt.Fprintf(os.Stderr, "Connecting to %s.onion...\n", toOnion[:16])
		connectCtx, connectCancel := context.WithTimeout(ctx, 120*time.Second)
		defer connectCancel()

		conn, err := node.DialTor(connectCtx, toOnion, 9000)
		if err != nil {
			message.SaveToOutbox(hollerDir, env)
			printOutboxHint(hollerDir)
			return nil
		}
		defer conn.Close()

		if err := node.SendTor(conn, env); err != nil {
			message.SaveToOutbox(hollerDir, env)
			fmt.Fprintf(os.Stderr, "Send failed — queued in outbox: %v\n", err)
			printOutboxHint(hollerDir)
			return nil
		}

		// Wait for ack and verify signature
		ack, err := node.RecvTor(conn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "No ack received (message likely delivered): %v\n", err)
		} else if ack.Type == "ack" && ack.Body == env.ID {
			if valid, verr := ack.Verify(); verr != nil || !valid {
				fmt.Fprintf(os.Stderr, "Ack signature invalid\n")
			}
		}

		if sentData, err := json.Marshal(env); err == nil {
			message.AppendToSent(hollerDir, sentData)
		}
		fmt.Fprintf(os.Stderr, "Message sent to %s.onion\n", toOnion[:16])
		return nil
	},
}

func printOutboxHint(hollerDir string) {
	if running, _, _ := daemon.IsRunning(hollerDir); running {
		fmt.Fprintf(os.Stderr, "Queued in outbox — daemon will retry delivery\n")
	} else {
		fmt.Fprintf(os.Stderr, "Queued in outbox — start daemon to auto-retry\n")
	}
}
