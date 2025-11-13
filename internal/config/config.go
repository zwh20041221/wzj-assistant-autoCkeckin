package config

import (
	"encoding/json"
	_ "errors"
	"fmt"
	"os"
)

type Config struct {
	Polling_interval     int     `json:"polling_interval"`
	Start_delay          int     `json:"start_delay"`
	Lat                  float64 `json:"lat"`
	Lon                  float64 `json:"lon"`
	Ua                   string  `json:"ua"`
	Max_polling_attempts int     `json:"max_polling_attempts"`
	Debug                int     `json:"debug"`
}

func Load() (*Config, error) {
	cfg_path := "config.json"
	data, err := os.ReadFile(cfg_path)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	cfg := &Config{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	if cfg.Polling_interval <= 0 {
		cfg.Polling_interval = 4000
	}
	if cfg.Start_delay <= 0 {
		cfg.Start_delay = 1000
	}
	if cfg.Max_polling_attempts <= 0 {
		cfg.Max_polling_attempts = 30
	}
	// default debug off
	if cfg.Debug != 1 {
		cfg.Debug = 0
	}
	return cfg, nil
}
