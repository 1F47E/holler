package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"

	"github.com/1F47E/holler/identity"
	"github.com/1F47E/holler/node"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(peersCmd)
}

var peersCmd = &cobra.Command{
	Use:   "peers",
	Short: "List discovered peers on DHT",
	RunE: func(cmd *cobra.Command, args []string) error {
		privKey, err := identity.LoadOrFail()
		if err != nil {
			return err
		}

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()

		var d *dht.IpfsDHT
		h, err := node.NewHost(ctx, privKey, &d)
		if err != nil {
			return err
		}
		defer h.Close()

		d, err = node.NewDHT(ctx, h)
		if err != nil {
			return err
		}
		defer d.Close()

		fmt.Fprintf(os.Stderr, "Discovering peers...\n")
		time.Sleep(5 * time.Second)

		rt := d.RoutingTable()
		peers := rt.ListPeers()

		if len(peers) == 0 {
			fmt.Fprintln(os.Stderr, "No peers found in routing table.")
			return nil
		}

		fmt.Fprintf(os.Stderr, "Found %d peer(s):\n", len(peers))
		for _, p := range peers {
			entry := map[string]string{"peer_id": p.String()}
			data, _ := json.Marshal(entry)
			fmt.Println(string(data))
		}
		return nil
	},
}
