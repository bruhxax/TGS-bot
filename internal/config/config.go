package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	TelegramToken    string
	PublicBaseURL    string
	DatabaseURL      string
	AppSecret        string
	RemnawaveURL     string
	RemnawaveToken   string
	ListenAddr       string
	LogLevel         string
	AdminTelegramIDs map[int64]bool
}

func Load() (Config, error) {
	c := Config{
		TelegramToken:    strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		PublicBaseURL:    strings.TrimRight(value("PUBLIC_BASE_URL", "https://tgs-bot.mooo.com"), "/"),
		DatabaseURL:      value("DATABASE_URL", "postgresql://tgsbot:tgsbot@db:5432/tgsbot"),
		AppSecret:        strings.TrimSpace(os.Getenv("APP_SECRET")),
		RemnawaveURL:     strings.TrimRight(strings.TrimSpace(os.Getenv("REMNAWAVE_URL")), "/"),
		RemnawaveToken:   strings.TrimSpace(os.Getenv("REMNAWAVE_TOKEN")),
		ListenAddr:       value("LISTEN_ADDR", ":8080"),
		LogLevel:         value("LOG_LEVEL", "INFO"),
		AdminTelegramIDs: map[int64]bool{},
	}
	for _, raw := range strings.Split(os.Getenv("ADMIN_TELEGRAM_IDS"), ",") {
		if id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64); err == nil && id > 0 {
			c.AdminTelegramIDs[id] = true
		}
	}
	if c.TelegramToken == "" {
		return Config{}, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}
	if len(c.AppSecret) < 24 {
		return Config{}, fmt.Errorf("APP_SECRET must contain at least 24 characters")
	}
	return c, nil
}

func value(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func (c Config) MiniAppURL(page string) string {
	if page == "" {
		return c.PublicBaseURL + "/app"
	}
	return c.PublicBaseURL + "/app?page=" + page
}
