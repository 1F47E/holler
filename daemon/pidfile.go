package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const pidFileName = "holler.pid"

// PidPath returns the path to the PID file.
func PidPath(dir string) string {
	return filepath.Join(dir, pidFileName)
}

// WritePid atomically writes the PID to the PID file.
func WritePid(dir string, pid int) error {
	path := PidPath(dir)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strconv.Itoa(pid)+"\n"), 0644); err != nil {
		return fmt.Errorf("write pid tmp: %w", err)
	}
	return os.Rename(tmp, path)
}

// ReadPid reads the PID from the PID file.
func ReadPid(dir string) (int, error) {
	data, err := os.ReadFile(PidPath(dir))
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parse pid: %w", err)
	}
	return pid, nil
}

// RemovePid removes the PID file.
func RemovePid(dir string) error {
	if err := os.Remove(PidPath(dir)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// IsRunning checks if the daemon is running. Returns running status, PID, and any error.
func IsRunning(dir string) (bool, int, error) {
	pid, err := ReadPid(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, 0, nil
		}
		return false, 0, err
	}
	// Check if process is alive
	if err := syscall.Kill(pid, 0); err != nil {
		// Process not running — stale PID file
		RemovePid(dir)
		return false, pid, nil
	}
	return true, pid, nil
}
