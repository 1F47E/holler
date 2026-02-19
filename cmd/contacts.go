package cmd

import (
	"fmt"

	"github.com/1F47E/holler/identity"
	"github.com/spf13/cobra"
)

func init() {
	contactsCmd.AddCommand(contactsAddCmd)
	contactsCmd.AddCommand(contactsRmCmd)
	rootCmd.AddCommand(contactsCmd)
}

var contactsCmd = &cobra.Command{
	Use:   "contacts",
	Short: "Manage contact aliases",
	RunE: func(cmd *cobra.Command, args []string) error {
		contacts, err := identity.LoadContacts()
		if err != nil {
			return err
		}
		if len(contacts) == 0 {
			fmt.Println("No contacts saved.")
			return nil
		}
		for _, alias := range contacts.SortedAliases() {
			fmt.Printf("%-20s %s.onion\n", alias, contacts[alias])
		}
		return nil
	},
}

var contactsAddCmd = &cobra.Command{
	Use:   "add <alias> <onion-addr>",
	Short: "Save a contact alias",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias, onionAddr := args[0], args[1]

		if !identity.ValidOnionAddr(onionAddr) {
			return fmt.Errorf("invalid onion address: must be 56 characters, lowercase a-z and 2-7")
		}

		contacts, err := identity.LoadContacts()
		if err != nil {
			return err
		}
		contacts[alias] = onionAddr
		if err := identity.SaveContacts(contacts); err != nil {
			return err
		}
		fmt.Printf("Added contact %q → %s.onion\n", alias, onionAddr[:16]+"...")
		return nil
	},
}

var contactsRmCmd = &cobra.Command{
	Use:   "rm <alias>",
	Short: "Remove a contact",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]

		contacts, err := identity.LoadContacts()
		if err != nil {
			return err
		}
		if _, ok := contacts[alias]; !ok {
			return fmt.Errorf("contact %q not found", alias)
		}
		delete(contacts, alias)
		if err := identity.SaveContacts(contacts); err != nil {
			return err
		}
		fmt.Printf("Removed contact %q\n", alias)
		return nil
	},
}
