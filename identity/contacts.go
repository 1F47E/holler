package identity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

var onionAddrRegex = regexp.MustCompile(`^[a-z2-7]{56}$`)

// ValidOnionAddr checks if s is a valid 56-char base32 v3 onion service ID.
func ValidOnionAddr(s string) bool {
	return onionAddrRegex.MatchString(s)
}

const contactsFile = "contacts.json"

// Contacts maps alias names to onion addresses (56-char base32, no .onion suffix).
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
// Supports migration from tor_contacts.json for existing users.
func LoadContacts() (Contacts, error) {
	path, err := ContactsPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// Try legacy tor_contacts.json
		legacyPath := filepath.Join(filepath.Dir(path), "tor_contacts.json")
		data, err = os.ReadFile(legacyPath)
		if os.IsNotExist(err) {
			return make(Contacts), nil
		}
		if err != nil {
			return nil, fmt.Errorf("read contacts: %w", err)
		}
		// Migrate: parse and save to new location
		var c Contacts
		if err := json.Unmarshal(data, &c); err != nil {
			return nil, fmt.Errorf("parse contacts: %w", err)
		}
		if saveErr := SaveContacts(c); saveErr == nil {
			os.Remove(legacyPath) // best effort cleanup
		}
		return c, nil
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

// Resolve tries to resolve an alias to an onion address.
// If the input is not a known alias, returns the input as-is (assumed to be a raw onion address).
func (c Contacts) Resolve(aliasOrOnion string) string {
	if addr, ok := c[aliasOrOnion]; ok {
		return addr
	}
	return aliasOrOnion
}

// FindByOnion does a reverse lookup: finds the alias for a given onion address.
func (c Contacts) FindByOnion(onionAddr string) (alias string, found bool) {
	for a, addr := range c {
		if addr == onionAddr {
			return a, true
		}
	}
	return "", false
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
