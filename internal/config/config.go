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
	Lat_W12              float64 `json:"lat_w12"`
	Lon_W12              float64 `json:"lon_w12"`
	Lat_S1               float64 `json:"lat_s1"`
	Lon_S1               float64 `json:"lon_s1"`
	Ua                   string  `json:"ua"`
	Max_polling_attempts int     `json:"max_polling_attempts"`
	Debug                int     `json:"debug"`
	AutoQRMode           string  `json:"autoqr_mode"`        // "manual" 或 "autohotkey"
	AutoQRIntervalMS     int     `json:"autoqr_interval_ms"` // 自动重扫间隔，毫秒
	AutoQRX              int     `json:"autoqr_x"`           // 二维码窗口左上角X
	AutoQRY              int     `json:"autoqr_y"`           // 二维码窗口左上角Y
	AutoQRSize           int     `json:"autoqr_size"`        // 二维码图片边长
	AutoQRRecognizeX     int     `json:"autoqr_recognize_x"` // 识别按钮绝对X（可选）
	AutoQRRecognizeY     int     `json:"autoqr_recognize_y"` // 识别按钮绝对Y（可选）
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
	// 默认启用 AutoHotkey 模式
	if cfg.AutoQRMode == "" {
		cfg.AutoQRMode = "autohotkey"
	}
	if cfg.AutoQRIntervalMS <= 0 {
		cfg.AutoQRIntervalMS = 4000
	}
	if cfg.AutoQRX <= 0 {
		cfg.AutoQRX = 420
	}
	if cfg.AutoQRY <= 0 {
		cfg.AutoQRY = 160
	}
	if cfg.AutoQRSize <= 0 {
		cfg.AutoQRSize = 320
	}
	// 默认识别按钮坐标（如不适配可在配置中修改或置为0禁用）
	if cfg.AutoQRRecognizeX <= 0 {
		cfg.AutoQRRecognizeX = 560
	}
	if cfg.AutoQRRecognizeY <= 0 {
		cfg.AutoQRRecognizeY = 520
	}
	return cfg, nil
}
