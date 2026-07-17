// Package secrets stores and retrieves database connection passwords in the
// OS-native credential store (macOS Keychain, Windows Credential Manager, or
// the Linux/BSD Secret Service API), so passwords don't need to live in
// plaintext in connections.json.
package secrets

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

// service is the keyring "service" namespace all database-mcp secrets are
// stored under; the connection ID is the account name within it.
const service = "database-mcp"

// Set stores the password for connID in the OS credential store.
func Set(connID, password string) error {
	if err := keyring.Set(service, connID, password); err != nil {
		return fmt.Errorf("secrets: store password for %q: %w", connID, err)
	}
	return nil
}

// Get retrieves the password for connID from the OS credential store.
func Get(connID string) (string, error) {
	pw, err := keyring.Get(service, connID)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", fmt.Errorf("no password stored for connection %q in the OS credential store (run: database-mcp secret set %s)", connID, connID)
		}
		return "", fmt.Errorf("secrets: retrieve password for %q: %w", connID, err)
	}
	return pw, nil
}

// Delete removes the stored password for connID, if any. Deleting a
// non-existent entry is not an error.
func Delete(connID string) error {
	if err := keyring.Delete(service, connID); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("secrets: delete password for %q: %w", connID, err)
	}
	return nil
}

// Available reports whether the OS credential store is usable in this
// environment. Headless Linux without a Secret Service provider (no GNOME
// Keyring/KWallet running) is the main case where this returns false; the
// wizard and config loader use it to fall back gracefully.
func Available() bool {
	const probeAccount = "__database-mcp-probe__"
	if err := keyring.Set(service, probeAccount, "probe"); err != nil {
		return false
	}
	_ = keyring.Delete(service, probeAccount)
	return true
}
