package main

import (
	"fmt"
	"os"

	"github.com/agung-ap/database-mcp/internal/secrets"
	"golang.org/x/term"
)

// runSecretCommand implements `database-mcp secret set|delete <connection-id>`,
// letting users store or remove a connection's password in the OS credential
// store without hand-editing connections.json.
func runSecretCommand(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: database-mcp secret <set|delete> <connection-id>")
		return 1
	}

	action, connID := args[0], args[1]
	switch action {
	case "set":
		fmt.Printf("Password for %q: ", connID)
		pw, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to read password: %v\n", err)
			return 1
		}
		if len(pw) == 0 {
			fmt.Fprintln(os.Stderr, "password must not be empty")
			return 1
		}
		if err := secrets.Set(connID, string(pw)); err != nil {
			fmt.Fprintf(os.Stderr, "failed to store password: %v\n", err)
			return 1
		}
		fmt.Printf("Stored password for %q in the OS credential store.\n", connID)
		fmt.Printf(`Set "password_source": "keyring" on that connection in connections.json (and remove the plaintext password) to use it.`)
		fmt.Println()
		return 0
	case "delete":
		if err := secrets.Delete(connID); err != nil {
			fmt.Fprintf(os.Stderr, "failed to delete password: %v\n", err)
			return 1
		}
		fmt.Printf("Deleted stored password for %q.\n", connID)
		return 0
	default:
		fmt.Fprintln(os.Stderr, "usage: database-mcp secret <set|delete> <connection-id>")
		return 1
	}
}
