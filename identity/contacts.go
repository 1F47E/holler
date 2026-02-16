package identity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const contactsFile = "contacts.json"

// Contacts maps alias names to PeerID strings.
type Contacts map[string]string

// ContactsPath returns the path to ~/.holler/contacts.json.
func ContactsPath() (string, error) {
	dir, err := HollerDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, contactsFile), nil
}

// LoadContacts reads contacts from disk. Returns empty map if file doesn't exist.
func LoadContacts() (Contacts, error) {
	path, err := ContactsPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return make(Contacts), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read contacts: %w", err)
	}
	var c Contacts
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse contacts: %w", err)
	}
	return c, nil
}

// SaveContacts writes contacts to disk.
func SaveContacts(c Contacts) error {
	path, err := ContactsPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal contacts: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// Resolve tries to resolve an alias to a PeerID string.
// If the input is not a known alias, it returns the input as-is (assumed to be a raw PeerID).
func (c Contacts) Resolve(aliasOrPeerID string) string {
	if pid, ok := c[aliasOrPeerID]; ok {
		return pid
	}
	return aliasOrPeerID
}

// SortedAliases returns alias names sorted alphabetically.
func (c Contacts) SortedAliases() []string {
	aliases := make([]string, 0, len(c))
	for a := range c {
		aliases = append(aliases, a)
	}
	sort.Strings(aliases)
	return aliases
}
