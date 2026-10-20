package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"
)

type Config struct {
	DatabaseURL   string
	HTTPAddr      string
	TelegramToken string
	LLMEndpoint   string
	LLMKey        string
	LLMModel      string
	Location      *time.Location
	LLMTimeout    time.Duration
}

func Load() (Config, error) {
	c := Config{
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		HTTPAddr:      os.Getenv("HTTP_ADDR"),
		TelegramToken: os.Getenv("TELEGRAM_TOKEN"),
		LLMEndpoint:   os.Getenv("LLM_ENDPOINT"),
		LLMKey:        os.Getenv("LLM_API_KEY"),
		LLMModel:      os.Getenv("LLM_MODEL"),
	}
	for key, value := range map[string]string{
		"TELEGRAM_TOKEN": c.TelegramToken,
		"LLM_ENDPOINT":   c.LLMEndpoint,
		"LLM_API_KEY":    c.LLMKey,
		"LLM_MODEL":      c.LLMModel,
	} {
		if value == "" {
			return c, fmt.Errorf("required setting: %s", key)
		}
	}
	u, err := url.Parse(c.LLMEndpoint)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
		return c, errors.New("invalid LLM_ENDPOINT")
	}
	if c.HTTPAddr == "" {
		c.HTTPAddr = ":8080"
	}
	zone := os.Getenv("FAMILY_TIMEZONE")
	if zone == "" {
		zone = "Asia/Yekaterinburg"
	}
	c.Location, err = time.LoadLocation(zone)
	if err != nil {
		return c, errors.New("invalid FAMILY_TIMEZONE")
	}
	timeout := os.Getenv("LLM_TIMEOUT")
	if timeout == "" {
		timeout = "30s"
	}
	c.LLMTimeout, err = time.ParseDuration(timeout)
	if err != nil || c.LLMTimeout <= 0 || c.LLMTimeout > 2*time.Minute {
		return c, errors.New("LLM_TIMEOUT must be between 0 and 2m")
	}
	return c, nil
}
