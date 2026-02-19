package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/1F47E/holler/daemon"
	"github.com/1F47E/holler/identity"
	"github.com/1F47E/holler/node"
	"github.com/spf13/cobra"
)

var daemonLogLines int
var daemonLogFollow bool

func init() {
	daemonLogCmd.Flags().IntVarP(&daemonLogLines, "lines", "n", 20, "Number of lines to show")
	daemonLogCmd.Flags().BoolVarP(&daemonLogFollow, "follow", "f", false, "Follow log output")

	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonCmd.AddCommand(daemonLogCmd)
	rootCmd.AddCommand(daemonCmd)
}

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the background listener daemon",
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the daemon in the background",
	RunE: func(cmd *cobra.Command, args []string) error {
		hollerDir, err := identity.HollerDir()
		if err != nil {
			return err
		}

		// Check if already running
		if running, pid, _ := daemon.IsRunning(hollerDir); running {
			return fmt.Errorf("daemon already running (PID %d)", pid)
		}

		// Validate tor_key exists
		if _, err := node.LoadOrCreateOnionKey(hollerDir); err != nil {
			return fmt.Errorf("no identity — run 'holler init' first: %w", err)
		}

		// Open log file
		logPath := logFilePath(hollerDir)
		logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("open log: %w", err)
		}

		// Build the daemon subprocess command
		execArgs := []string{"_run-daemon"}
		if identity.DirOverride != "" {
			execArgs = append(execArgs, "--dir", identity.DirOverride)
		}
		if node.Verbose {
			execArgs = append(execArgs, "--verbose")
		}

		daemonCmd := exec.Command(os.Args[0], execArgs...)
		daemonCmd.Stdout = logFile
		daemonCmd.Stderr = logFile
		daemonCmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

		if err := daemonCmd.Start(); err != nil {
			logFile.Close()
			return fmt.Errorf("start daemon: %w", err)
		}
		logFile.Close()

		pid := daemonCmd.Process.Pid
		if err := daemon.WritePid(hollerDir, pid); err != nil {
			return fmt.Errorf("write pid: %w", err)
		}

		// Verify the process is still alive after a brief wait
		time.Sleep(1 * time.Second)
		if err := syscall.Kill(pid, 0); err != nil {
			daemon.RemovePid(hollerDir)
			return fmt.Errorf("daemon exited immediately — check %s", logPath)
		}

		fmt.Printf("Daemon started (PID %d)\n", pid)
		fmt.Printf("Log: %s\n", logPath)
		return nil
	},
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		hollerDir, err := identity.HollerDir()
		if err != nil {
			return err
		}

		running, pid, err := daemon.IsRunning(hollerDir)
		if err != nil {
			return err
		}
		if !running {
			fmt.Println("Daemon is not running.")
			return nil
		}

		// Send SIGTERM
		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
			return fmt.Errorf("kill daemon: %w", err)
		}

		// Wait up to 5 seconds for graceful shutdown
		for i := 0; i < 50; i++ {
			time.Sleep(100 * time.Millisecond)
			if err := syscall.Kill(pid, 0); err != nil {
				daemon.RemovePid(hollerDir)
				fmt.Printf("Daemon stopped (PID %d)\n", pid)
				return nil
			}
		}

		// Force kill
		syscall.Kill(pid, syscall.SIGKILL)
		time.Sleep(200 * time.Millisecond)
		daemon.RemovePid(hollerDir)
		fmt.Printf("Daemon killed (PID %d)\n", pid)
		return nil
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	RunE: func(cmd *cobra.Command, args []string) error {
		hollerDir, err := identity.HollerDir()
		if err != nil {
			return err
		}

		running, pid, err := daemon.IsRunning(hollerDir)
		if err != nil {
			return err
		}

		if running {
			fmt.Printf("Daemon: running (PID %d)\n", pid)
		} else {
			fmt.Println("Daemon: stopped")
		}

		// Show last 5 lines of log
		logPath := logFilePath(hollerDir)
		if lines := tailFile(logPath, 5); len(lines) > 0 {
			fmt.Println("\nRecent log:")
			for _, line := range lines {
				fmt.Println("  " + line)
			}
		}
		return nil
	},
}

var daemonLogCmd = &cobra.Command{
	Use:   "log",
	Short: "View daemon log",
	RunE: func(cmd *cobra.Command, args []string) error {
		hollerDir, err := identity.HollerDir()
		if err != nil {
			return err
		}
		logPath := logFilePath(hollerDir)

		if daemonLogFollow {
			// Follow mode — exec tail -f
			tailCmd := exec.Command("tail", "-f", "-n", fmt.Sprintf("%d", daemonLogLines), logPath)
			tailCmd.Stdout = os.Stdout
			tailCmd.Stderr = os.Stderr
			return tailCmd.Run()
		}

		lines := tailFile(logPath, daemonLogLines)
		if len(lines) == 0 {
			fmt.Println("No log entries.")
			return nil
		}
		for _, line := range lines {
			fmt.Println(line)
		}
		return nil
	},
}

func logFilePath(hollerDir string) string {
	return hollerDir + "/holler.log"
}

func tailFile(path string, n int) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}
