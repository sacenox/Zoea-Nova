package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Credentials holds API keys for LLM providers.
type Credentials struct {
	Providers map[string]ProviderCredentials `json:"providers"`
}

// ProviderCredentials holds authentication for a single provider.
type ProviderCredentials struct {
	APIKey string `json:"api_key"`
}

// LoadCredentials reads credentials from ~/.zoea-nova/credentials.json.
func LoadCredentials() (*Credentials, error) {
	path, err := credentialsPath()
	if err != nil {
		return nil, err
	}

	creds := &Credentials{
		Providers: make(map[string]ProviderCredentials),
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return creds, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, creds); err != nil {
		return nil, err
	}

	return creds, nil
}

// SaveCredentials writes credentials to ~/.zoea-nova/credentials.json with 0600 permissions.
func SaveCredentials(creds *Credentials) error {
	dir, err := EnsureDataDir()
	if err != nil {
		return err
	}

	path := filepath.Join(dir, "credentials.json")
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// GetAPIKey returns the API key for a given provider, or empty string if not set.
func (c *Credentials) GetAPIKey(provider string) string {
	if c == nil || c.Providers == nil {
		return ""
	}
	return c.Providers[provider].APIKey
}

// SetAPIKey sets the API key for a given provider.
func (c *Credentials) SetAPIKey(provider, apiKey string) {
	if c.Providers == nil {
		c.Providers = make(map[string]ProviderCredentials)
	}
	c.Providers[provider] = ProviderCredentials{APIKey: apiKey}
}

func credentialsPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "credentials.json"), nil
}
