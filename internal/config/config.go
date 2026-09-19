package config

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
)

// ServerConfig holds required HTTP server settings.
type ServerConfig struct {
	Port string `validate:"required"`
}

type DatabaseConfig struct {
	URL string `validate:"required"`
}

type BotConfig struct {
	Name string `validate:"required"`
}

type SplitBotConfig struct {
	URL string `validate:"required,url"`
}

type NanobotConfig struct {
	URL string `validate:"required,url"`
}

type HermesConfig struct {
	URL    string `validate:"required,url"` // HERMES_URL e.g. http://hermes:8765
	APIKey string // HERMES_API_KEY optional shared secret
}

// GrafanaConfig holds optional Grafana webhook defaults.
type GrafanaConfig struct {
	AlertGroupID string
}

type GoogleConfig struct {
	ServiceAccountJSON string // GOOGLE_SERVICE_ACCOUNT_JSON (raw JSON or file path)
	CredentialsFile    string // GOOGLE_APPLICATION_CREDENTIALS
	DriveFolderID      string // GOOGLE_DRIVE_FOLDER_ID
}

// Config is the root application configuration.
type Config struct {
	Server   ServerConfig   `validate:"required"`
	Database DatabaseConfig `validate:"required"`
	Bot      BotConfig      `validate:"required"`
	SplitBot SplitBotConfig `validate:"required"`
	Nanobot  NanobotConfig  `validate:"required"`
	Hermes   HermesConfig   `validate:"required"`
	Grafana  GrafanaConfig
	Google   GoogleConfig
}

var (
	instance *Config
	once     sync.Once
)

func fromEnv() *Config {
	return &Config{
		Server: ServerConfig{
			Port: strings.TrimSpace(os.Getenv("PORT")),
		},
		Database: DatabaseConfig{
			URL: strings.TrimSpace(os.Getenv("DATABASE_URL")),
		},
		Bot: BotConfig{
			Name: strings.TrimSpace(os.Getenv("BOT_NAME")),
		},
		SplitBot: SplitBotConfig{
			URL: strings.TrimSpace(os.Getenv("SPLIT_BOT_URL")),
		},
		Nanobot: NanobotConfig{
			URL: strings.TrimSpace(os.Getenv("NANOBOT_URL")),
		},
		Hermes: HermesConfig{
			URL:    strings.TrimSpace(os.Getenv("HERMES_URL")),
			APIKey: strings.TrimSpace(os.Getenv("HERMES_API_KEY")),
		},
		Grafana: GrafanaConfig{
			AlertGroupID: strings.TrimSpace(os.Getenv("GRAFANA_ALERT_GROUP_ID")),
		},
		Google: GoogleConfig{
			ServiceAccountJSON: strings.TrimSpace(os.Getenv("GOOGLE_SERVICE_ACCOUNT_JSON")),
			CredentialsFile:    strings.TrimSpace(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")),
			DriveFolderID:      strings.TrimSpace(os.Getenv("GOOGLE_DRIVE_FOLDER_ID")),
		},
	}
}

// MustLoad loads .env if present, reads environment into a [Config], validates it, and stores a singleton.
// It panics if .env exists but cannot be read, or if validation fails (e.g. missing required fields).
func MustLoad() *Config {
	once.Do(func() {
		if err := loadDotEnv(); err != nil {
			panic(err)
		}
		cfg := fromEnv()
		applyDefaults(cfg)
		if err := validateConfig(cfg); err != nil {
			panic(fmt.Errorf("config validation failed: %w", err))
		}
		instance = cfg
	})
	return instance
}

// Get returns the loaded config singleton. Panics if [MustLoad] was not called first.
func Get() *Config {
	if instance == nil {
		panic("config not loaded: call config.MustLoad first")
	}
	return instance
}

func loadDotEnv() error {
	if err := godotenv.Load(); err != nil {
		if !os.IsNotExist(err) {
			log.Printf("warning: could not load .env: %v", err)
			return err
		}
	}
	return nil
}

func applyDefaults(cfg *Config) {
	if cfg.Server.Port == "" {
		cfg.Server.Port = "8080"
	}
}

func validateConfig(cfg *Config) error {
	v := validator.New()
	return v.Struct(cfg)
}
