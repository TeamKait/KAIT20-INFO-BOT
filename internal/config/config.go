package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all runtime settings. Everything comes from environment
// variables; .env is loaded when present (local runs, docker compose).
type Config struct {
	Token        string
	APIURL       string
	VaultURL     string
	VaultBranch  string
	VaultDir     string
	ImagesDir    string
	GitToken     string
	SyncInterval time.Duration
	AllowedChats map[int64]struct{}
	StartTime    time.Time
	LogLevel     slog.Level
}

// Load reads and validates the configuration.
func Load() (*Config, error) {
	// Missing .env is fine: docker compose passes variables via env_file.
	_ = godotenv.Load()

	c := &Config{
		Token:       strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		APIURL:      strings.TrimRight(envOr("TELEGRAM_API_URL", "https://api.telegram.org"), "/"),
		VaultURL:    envOr("VAULT_REPO_URL", "https://github.com/TeamKait/KAIT20-INFO.git"),
		VaultBranch: envOr("VAULT_BRANCH", "master"),
		VaultDir:    envOr("VAULT_DIR", "/data/vault"),
		ImagesDir:   envOr("IMAGES_DIR", "Прочее/Картинки"),
		GitToken:    strings.TrimSpace(os.Getenv("GIT_TOKEN")),
		StartTime:   time.Now(),
	}

	if c.Token == "" {
		return nil, errors.New("TELEGRAM_BOT_TOKEN is not set")
	}

	rawInterval := envOr("SYNC_INTERVAL", "1h")
	interval, err := time.ParseDuration(rawInterval)
	if err != nil || interval < time.Minute {
		return nil, fmt.Errorf("SYNC_INTERVAL %q: want a duration of at least 1m (for example 1h)", rawInterval)
	}
	c.SyncInterval = interval

	c.AllowedChats, err = parseChatIDs(os.Getenv("ALLOWED_CHAT_IDS"))
	if err != nil {
		return nil, err
	}

	c.LogLevel, err = parseLevel(envOr("LOG_LEVEL", "info"))
	if err != nil {
		return nil, err
	}

	dir, err := filepath.Abs(c.VaultDir)
	if err != nil {
		return nil, fmt.Errorf("VAULT_DIR: %w", err)
	}
	c.VaultDir = dir

	return c, nil
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func parseChatIDs(raw string) (map[int64]struct{}, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	ids := make(map[int64]struct{})
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("ALLOWED_CHAT_IDS: %q is not a chat id", part)
		}
		ids[id] = struct{}{}
	}
	return ids, nil
}

func parseLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	}
	return 0, fmt.Errorf("LOG_LEVEL %q: want debug, info, warn or error", raw)
}
