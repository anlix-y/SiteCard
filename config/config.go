package config

import (
	"encoding/json"
	"os"
)

// File-based configuration loader. Reads config/config.json if present
// and exports values into environment variables to keep the rest of
// the app unchanged. Keys map to env vars as follows:
//
//	database_url  -> DATABASE_URL
//	jwt_secret    -> JWT_SECRET
//	log           -> LOG ("1" to enable)
//	log_level     -> LOG_LEVEL (debug|info|error|off)
//	github_tokens -> GITHUB_TOKENS (comma separated)
//	github_proxy  -> GITHUB_PROXY (e.g. socks5://127.0.0.1:9050)
//	listen        -> LISTEN (e.g. :8080)
type cfg struct {
	DatabaseURL  string `json:"database_url"`
	JWTSecret    string `json:"jwt_secret"`
	Log          string `json:"log"`
	LogLevel     string `json:"log_level"`
	GitHubTokens string `json:"github_tokens"`
	GitHubProxy  string `json:"github_proxy"`
	Listen       string `json:"listen"`
}

func setEnvIfNotEmpty(key, val string) {
	if val == "" {
		return
	}
	_ = os.Setenv(key, val)
}

// Load reads config/config.json and exports values to environment.
// It is safe to call when the file does not exist.
func Load() {
	f, err := os.Open("config/config.json")
	if err != nil {
		return
	}
	defer f.Close()
	var c cfg
	if err := json.NewDecoder(f).Decode(&c); err != nil {
		return
	}
	setEnvIfNotEmpty("DATABASE_URL", c.DatabaseURL)
	setEnvIfNotEmpty("JWT_SECRET", c.JWTSecret)
	setEnvIfNotEmpty("LOG", c.Log)
	setEnvIfNotEmpty("LOG_LEVEL", c.LogLevel)
	setEnvIfNotEmpty("GITHUB_TOKENS", c.GitHubTokens)
	setEnvIfNotEmpty("GITHUB_PROXY", c.GitHubProxy)
	setEnvIfNotEmpty("LISTEN", c.Listen)
}
