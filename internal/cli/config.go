package cli

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type savedWorkspace struct {
	ID        string `json:"id"`
	Handle    string `json:"handle,omitempty"`
	AccountID string `json:"accountId,omitempty"`
	Role      string `json:"role,omitempty"`
}

type cliConfig struct {
	DefaultWorkspace *savedWorkspace `json:"defaultWorkspace,omitempty"`
	LastLoginEmail   string          `json:"lastLoginEmail,omitempty"`
}

type storedCredentials struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresAt    string `json:"expiresAt"`
	ClientID     string `json:"clientId"`
	UserID       string `json:"userId"`
	AccountID    string `json:"accountId,omitempty"`
}

func configDir() string {
	if override := strings.TrimSpace(os.Getenv("EPISMO_CONFIG_DIR")); override != "" {
		return override
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".epismo"
	}
	return filepath.Join(home, ".epismo")
}

func configPath() string      { return filepath.Join(configDir(), "config") }
func credentialsPath() string { return filepath.Join(configDir(), "credentials") }

func readJSONFile(path string, value any) (bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(data, value); err != nil {
		return false, err
	}
	return true, nil
}

func readConfig() (cliConfig, error) {
	var raw struct {
		DefaultWorkspace *struct {
			ID        string `json:"id"`
			Handle    string `json:"handle"`
			Name      string `json:"name"`
			AccountID string `json:"accountId"`
			Role      string `json:"role"`
		} `json:"defaultWorkspace"`
		LastLoginEmail string `json:"lastLoginEmail"`
	}
	_, err := readJSONFile(configPath(), &raw)
	config := cliConfig{LastLoginEmail: raw.LastLoginEmail}
	if raw.DefaultWorkspace != nil {
		handle := raw.DefaultWorkspace.Handle
		if strings.TrimSpace(handle) == "" {
			handle = raw.DefaultWorkspace.Name
		}
		config.DefaultWorkspace = &savedWorkspace{ID: raw.DefaultWorkspace.ID, Handle: handle, AccountID: raw.DefaultWorkspace.AccountID, Role: raw.DefaultWorkspace.Role}
	}
	if config.DefaultWorkspace != nil {
		config.DefaultWorkspace.ID = strings.TrimSpace(config.DefaultWorkspace.ID)
		config.DefaultWorkspace.Handle = strings.TrimSpace(config.DefaultWorkspace.Handle)
		config.DefaultWorkspace.AccountID = strings.TrimSpace(config.DefaultWorkspace.AccountID)
		config.DefaultWorkspace.Role = strings.TrimSpace(config.DefaultWorkspace.Role)
		if config.DefaultWorkspace.ID == "" {
			config.DefaultWorkspace = nil
		}
	}
	config.LastLoginEmail = strings.TrimSpace(config.LastLoginEmail)
	if err != nil {
		return config, &Error{Code: "CONFIG_READ_FAILED", Message: "Failed to read CLI configuration.", Hint: "Check that the configuration file contains valid JSON and is readable.", Details: map[string]any{"path": configPath()}, ExitCode: 1, Cause: err}
	}
	return config, nil
}

func writeJSONAtomic(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".epismo-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil && !errors.Is(err, fs.ErrPermission) {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return replaceFile(tmpPath, path)
}

func writeConfig(config cliConfig) error {
	if err := writeJSONAtomic(configPath(), config); err != nil {
		return &Error{Code: "CONFIG_WRITE_FAILED", Message: "Failed to save CLI configuration.", Hint: "Check that the configuration directory is writable.", Details: map[string]any{"path": configPath()}, ExitCode: 1, Cause: err}
	}
	return nil
}

func readCredentials() (*storedCredentials, error) {
	var credentials storedCredentials
	found, err := readJSONFile(credentialsPath(), &credentials)
	if err != nil {
		return nil, &Error{Code: "CREDENTIALS_READ_FAILED", Message: "Failed to read saved credentials.", Hint: "Check that the credentials file contains valid JSON and is readable.", Details: map[string]any{"path": credentialsPath()}, ExitCode: 1, Cause: err}
	}
	if !found {
		return nil, nil
	}
	if credentials.AccessToken == "" || credentials.RefreshToken == "" || credentials.ExpiresAt == "" || credentials.UserID == "" {
		return nil, nil
	}
	return &credentials, nil
}

func writeCredentials(credentials storedCredentials) error {
	if err := writeJSONAtomic(credentialsPath(), credentials); err != nil {
		return &Error{Code: "CREDENTIALS_WRITE_FAILED", Message: "Failed to save credentials.", Hint: "Check that the credentials directory is writable.", Details: map[string]any{"path": credentialsPath()}, ExitCode: 1, Cause: err}
	}
	return nil
}

func clearCredentials() error {
	if err := writeJSONAtomic(credentialsPath(), map[string]any{}); err != nil {
		return &Error{Code: "CREDENTIALS_CLEAR_FAILED", Message: "Failed to clear saved credentials.", Hint: "Check that the credentials directory is writable.", Details: map[string]any{"path": credentialsPath()}, ExitCode: 1, Cause: err}
	}
	return nil
}

func apiURL() string {
	if override := strings.TrimRight(strings.TrimSpace(os.Getenv("EPISMO_API_URL")), "/"); override != "" {
		return override
	}
	if os.Getenv("APP_ENV") == "dev" {
		return "http://localhost:8000"
	}
	return "https://api.epismo.ai"
}

func webURL() string {
	if override := strings.TrimRight(strings.TrimSpace(os.Getenv("EPISMO_WEB_URL")), "/"); override != "" {
		return override
	}
	if os.Getenv("APP_ENV") == "dev" {
		return "http://localhost:5173"
	}
	return "https://epismo.ai"
}
