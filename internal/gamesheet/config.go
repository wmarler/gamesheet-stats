package gamesheet

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Defaults for the production GameSheet Teams deployment. These are public
// client-side values embedded in the teams.gamesheet.app web bundle; they are
// not secrets. They can be overridden in config.json if GameSheet changes them.
const (
	DefaultAPIBase        = "https://api.teams.gamesheet.app"
	DefaultFirebaseAPIKey = "AIzaSyCk5pKBFxvCMuwPchzXgvvz4XmmscJTvs8"
)

// Config holds the user-supplied settings. It is loaded from a JSON file that
// must be kept out of version control because it contains credentials.
type Config struct {
	// Email and Password are the credentials used to sign in to
	// teams.gamesheet.app (Firebase email/password auth).
	Email    string `json:"email"`
	Password string `json:"password"`

	// TeamIDs lists the GameSheet "prototeam" UUIDs to report on. When empty,
	// every non-archived team visible to the account is used.
	TeamIDs []string `json:"team_ids"`

	// Optional overrides.
	APIBase        string `json:"api_base,omitempty"`
	FirebaseAPIKey string `json:"firebase_api_key,omitempty"`
}

// LoadConfig reads and validates a config file.
func LoadConfig(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w (copy config.example.json to config.json and fill it in)", path, err)
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	c.Email = strings.TrimSpace(c.Email)
	if c.Email == "" || c.Password == "" {
		return nil, errors.New("config: email and password are required")
	}
	if c.APIBase == "" {
		c.APIBase = DefaultAPIBase
	}
	c.APIBase = strings.TrimRight(c.APIBase, "/")
	if c.FirebaseAPIKey == "" {
		c.FirebaseAPIKey = DefaultFirebaseAPIKey
	}
	return &c, nil
}
