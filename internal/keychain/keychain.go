// Package keychain provides a thin, provider-agnostic wrapper around the
// OS native secret store (macOS Keychain, Windows Credential Manager,
// Linux Secret Service via D-Bus).
package keychain

import (
	"fmt"

	"github.com/zalando/go-keyring"
)

const service = "flow-cli"

// Keychain is an injectable secret-store client. Use New() to obtain one.
// Defining it as a struct (rather than bare package functions) allows it to
// be passed as a dependency and swapped out in tests.
type Keychain struct{}

// New returns a Keychain backed by the OS native secret store.
func New() *Keychain { return &Keychain{} }

// Set stores a secret for the given provider and account.
func (k *Keychain) Set(provider, account, secret string) error {
	return Set(provider, account, secret)
}

// Get retrieves the secret for the given provider and account.
func (k *Keychain) Get(provider, account string) (string, error) {
	return Get(provider, account)
}

// Delete removes the stored secret for the given provider and account.
func (k *Keychain) Delete(provider, account string) error {
	return Delete(provider, account)
}

// ── Package-level helpers (used by the struct methods and cmd layer) ──────────

// Set stores a secret for the given provider and account.
// The provider name is incorporated into the service key so multiple
// providers never collide (e.g. "flow-cli/jira", "flow-cli/github").
func Set(provider, account, secret string) error {
	if err := keyring.Set(svc(provider), account, secret); err != nil {
		return fmt.Errorf("storing %s credential for %s: %w", provider, account, err)
	}
	return nil
}

// Get retrieves the secret for the given provider and account.
func Get(provider, account string) (string, error) {
	secret, err := keyring.Get(svc(provider), account)
	if err != nil {
		return "", fmt.Errorf("retrieving %s credential for %s: %w", provider, account, err)
	}
	return secret, nil
}

// Delete removes the stored secret for the given provider and account.
func Delete(provider, account string) error {
	if err := keyring.Delete(svc(provider), account); err != nil {
		return fmt.Errorf("deleting %s credential for %s: %w", provider, account, err)
	}
	return nil
}

func svc(provider string) string { return service + "/" + provider }
