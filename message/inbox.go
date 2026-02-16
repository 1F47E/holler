package message

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	inboxFile = "inbox.jsonl"
	sentFile  = "sent.jsonl"
)

// InboxPath returns the path to ~/.holler/inbox.jsonl.
func InboxPath(hollerDir string) string {
	return filepath.Join(hollerDir, inboxFile)
}

// AppendToInbox appends a JSON-encoded envelope line to the inbox file.
func AppendToInbox(hollerDir string, data []byte) error {
	return appendToFile(filepath.Join(hollerDir, inboxFile), data)
}

// AppendToSent appends a JSON-encoded envelope line to the sent log.
func AppendToSent(hollerDir string, data []byte) error {
	return appendToFile(filepath.Join(hollerDir, sentFile), data)
}

func appendToFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open %s: %w", filepath.Base(path), err)
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write to %s: %w", filepath.Base(path), err)
	}
	return nil
}
