package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const Version = "1.1.0"

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("holler v%s\n", Version)
		checkLatestVersion()
	},
}

func checkLatestVersion() {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/1F47E/holler/releases/latest")
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}
	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return
	}
	latest := strings.TrimPrefix(release.TagName, "v")
	if latest != "" && latest != Version {
		fmt.Printf("\n  update available: v%s → v%s\n", Version, latest)
		fmt.Println("  https://github.com/1F47E/holler/releases/latest")
	}
}
