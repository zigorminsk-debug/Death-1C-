package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigSaveAndLoadRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "Death1C", "config.json")

	loaded, err := LoadFrom(cfgFile)
	if err != nil {
		t.Fatalf("LoadFrom missing file returned error: %v", err)
	}
	if loaded.Configured {
		t.Fatalf("expected Configured=false for new config")
	}
	if !loaded.ProtectDesigner {
		t.Fatalf("expected ProtectDesigner=true by default")
	}

	loaded.ConnectionMode = ModeRAC
	loaded.ServerAddr = "srv-1c.local"
	loaded.ClusterUser = "АдминистраторКластера"
	loaded.ClusterPassword = "Сложный$Пароль!2026"
	loaded.GroupByServer = true

	if err := loaded.SaveTo(cfgFile); err != nil {
		t.Fatalf("SaveTo failed: %v", err)
	}

	rawBytes, err := os.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if strings.Contains(string(rawBytes), "Сложный$Пароль!2026") {
		t.Fatalf("plaintext password must never be stored in config.json")
	}

	reloaded, err := LoadFrom(cfgFile)
	if err != nil {
		t.Fatalf("LoadFrom saved file failed: %v", err)
	}
	if !reloaded.Configured {
		t.Fatalf("expected Configured=true after SaveTo")
	}
	if reloaded.ClusterUser != "АдминистраторКластера" {
		t.Fatalf("unexpected ClusterUser: %q", reloaded.ClusterUser)
	}
	if reloaded.ClusterPassword != "Сложный$Пароль!2026" {
		t.Fatalf("decrypted password mismatch: got %q", reloaded.ClusterPassword)
	}
	if reloaded.EffectiveRASAddr() != "srv-1c.local:1545" {
		t.Fatalf("unexpected EffectiveRASAddr: %q", reloaded.EffectiveRASAddr())
	}
}

func TestEffectiveAddresses(t *testing.T) {
	c := Config{ServerAddr: "tcp://10.0.0.5:1545"}
	if got := c.EffectiveRASAddr(); got != "10.0.0.5:1545" {
		t.Fatalf("got %q, want 10.0.0.5:1545", got)
	}

	c2 := Config{ServerAddr: "10.0.0.5:1540"}
	if got := c2.EffectiveAgentAddr(); got != "10.0.0.5" {
		t.Fatalf("got %q, want 10.0.0.5", got)
	}

	c3 := Config{ServerAddr: "10.0.0.5:2540"}
	if got := c3.EffectiveAgentAddr(); got != "tcp://10.0.0.5:2540" {
		t.Fatalf("got %q, want tcp://10.0.0.5:2540", got)
	}
}
