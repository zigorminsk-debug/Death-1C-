package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

const (
	ModeRAC = "rac"
	ModeCOM = "com"

	DefaultRASPort   = "1545"
	DefaultAgentPort = "1540"
)

// Config holds persisted 1C server connection settings and administrator credentials.
type Config struct {
	ConnectionMode      string `json:"connection_mode"`
	ServerAddr          string `json:"server_addr"`
	RACPath             string `json:"rac_path"`
	ClusterUser         string `json:"cluster_user"`
	ClusterPasswordEnc  string `json:"cluster_password_enc,omitempty"`
	ClusterPassword     string `json:"-"`
	GroupByServer       bool   `json:"group_by_server"`
	ProtectDesigner     bool   `json:"protect_designer"`
	ConfirmBeforeDelete bool   `json:"confirm_before_delete"`
	Configured          bool   `json:"configured"`
}

// Default returns a new Config initialized with safe defaults for 1C:Enterprise Server.
func Default() Config {
	return Config{
		ConnectionMode:      ModeRAC,
		ServerAddr:          "localhost:" + DefaultRASPort,
		ProtectDesigner:     true,
		ConfirmBeforeDelete: false,
		Configured:          false,
	}
}

// DefaultConfigPath returns the path to %AppData%\Death1C\config.json on Windows
// (or the OS-appropriate user config directory).
func DefaultConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return "", fmt.Errorf("не удалось определить каталог настроек пользователя: %w", err)
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "Death1C", "config.json"), nil
}

// Load reads Config from DefaultConfigPath. If the file does not exist yet,
// Default() is returned without error.
func Load() (Config, error) {
	path, err := DefaultConfigPath()
	if err != nil {
		return Default(), err
	}
	return LoadFrom(path)
}

// LoadFrom reads Config from a specific file path.
func LoadFrom(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("ошибка чтения файла настроек %s: %w", path, err)
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return Default(), fmt.Errorf("повреждён файл настроек %s: %w", path, err)
	}

	if cfg.ConnectionMode != ModeRAC && cfg.ConnectionMode != ModeCOM {
		cfg.ConnectionMode = ModeRAC
	}
	if strings.TrimSpace(cfg.ServerAddr) == "" {
		if cfg.ConnectionMode == ModeCOM {
			cfg.ServerAddr = "localhost:" + DefaultAgentPort
		} else {
			cfg.ServerAddr = "localhost:" + DefaultRASPort
		}
	}

	if cfg.ClusterPasswordEnc != "" {
		pwd, decErr := decryptSecret(cfg.ClusterPasswordEnc)
		if decErr != nil {
			return cfg, decErr
		}
		cfg.ClusterPassword = pwd
	}

	return cfg, nil
}

// Save encrypts the administrator password and atomically persists Config to DefaultConfigPath.
func (c *Config) Save() error {
	path, err := DefaultConfigPath()
	if err != nil {
		return err
	}
	return c.SaveTo(path)
}

// SaveTo encrypts the administrator password and atomically writes Config to path.
func (c *Config) SaveTo(path string) error {
	c.ServerAddr = strings.TrimSpace(c.ServerAddr)
	c.RACPath = strings.TrimSpace(c.RACPath)
	c.ClusterUser = strings.TrimSpace(c.ClusterUser)
	if c.ConnectionMode != ModeRAC && c.ConnectionMode != ModeCOM {
		c.ConnectionMode = ModeRAC
	}

	encPwd, err := encryptSecret(c.ClusterPassword)
	if err != nil {
		return err
	}
	c.ClusterPasswordEnc = encPwd
	c.Configured = true

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("не удалось создать каталог настроек: %w", err)
	}

	payload, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("ошибка сериализации настроек: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, payload, 0o600); err != nil {
		return fmt.Errorf("ошибка записи файла настроек: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(path)
		if renameErr := os.Rename(tmpPath, path); renameErr != nil {
			return fmt.Errorf("не удалось сохранить файл настроек: %w", renameErr)
		}
	}
	return nil
}

// EffectiveRASAddr returns normalized host:port for 1C RAS (default port 1545).
func (c Config) EffectiveRASAddr() string {
	addr := strings.TrimSpace(c.ServerAddr)
	addr = strings.TrimPrefix(addr, "tcp://")
	addr = strings.TrimPrefix(addr, "http://")
	addr = strings.TrimSuffix(addr, "/")
	if addr == "" {
		return "localhost:" + DefaultRASPort
	}
	if _, _, err := net.SplitHostPort(addr); err == nil {
		return addr
	}
	if strings.HasPrefix(addr, "[") && strings.HasSuffix(addr, "]") {
		return addr + ":" + DefaultRASPort
	}
	if strings.Count(addr, ":") > 1 {
		return "[" + addr + "]:" + DefaultRASPort
	}
	return addr + ":" + DefaultRASPort
}

// EffectiveAgentAddr returns normalized server agent address for V83.COMConnector (default port 1540).
func (c Config) EffectiveAgentAddr() string {
	addr := strings.TrimSpace(c.ServerAddr)
	if addr == "" {
		return "localhost"
	}
	clean := strings.TrimPrefix(addr, "tcp://")
	if host, port, err := net.SplitHostPort(clean); err == nil {
		if port == "" || port == DefaultAgentPort {
			return host
		}
		return fmt.Sprintf("tcp://%s:%s", host, port)
	}
	return clean
}
