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

const (
	healthCheckInterval = 30 * time.Second
	reconnectMin        = 5 * time.Second
	reconnectMax        = 60 * time.Second
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

		msgHandler := func(env *message.Envelope) {
			data, err := json.Marshal(env)
			if err != nil {
				return
			}
			message.AppendToInbox(hollerDir, data)
			daemon.RunReceiveHook(hollerDir, env)
		}

		profile := node.LoadProfile(hollerDir)
		backoff := reconnectMin

		for {
			select {
			case <-ctx.Done():
				goto shutdown
			default:
			}

			tn, err := node.ListenTor(onionKey, onionAddr)
			if err != nil {
				logDaemon("tor connect failed: %v (retry in %s)", err, backoff)
				select {
				case <-ctx.Done():
					goto shutdown
				case <-time.After(backoff):
				}
				backoff = backoff * 2
				if backoff > reconnectMax {
					backoff = reconnectMax
				}
				continue
			}

			backoff = reconnectMin
			logDaemon("daemon started: %s.onion:9000", onionAddr)

			// Run one session until Tor health check fails or shutdown.
			func() {
				sessCtx, sessCancel := context.WithCancel(ctx)
				defer sessCancel()
				defer tn.Close() //nolint:errcheck

				go node.HandleTorConnections(sessCtx, tn, kp, msgHandler)
				go node.StartHomepage(sessCtx, tn.HTTPListener(), node.HomepageData{
					Name:      profile.Name,
					Bio:       profile.Bio,
					OnionAddr: onionAddr,
					Version:   Version,
				})
				go retryOutboxLoop(sessCtx, hollerDir)

				// Health check loop — blocks until failure or shutdown
				ticker := time.NewTicker(healthCheckInterval)
				defer ticker.Stop()
				for {
					select {
					case <-sessCtx.Done():
						return
					case <-ticker.C:
						if err := tn.Ping(); err != nil {
							logDaemon("tor health check failed: %v — reconnecting", err)
							return
						}
					}
				}
			}()
		}

	shutdown:
		logDaemon("daemon shutting down")
		daemon.RemovePid(hollerDir)
		return nil
	},
}

func logDaemon(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), msg)
}
