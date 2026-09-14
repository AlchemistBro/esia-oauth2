package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		path := writeTestConfig(t, `{
			"redirect_uri":"https://example.test/callback",
			"mnemonic":"TEST",
			"esia_uri":"https://esia.example.test",
			"csp_test_path":"/opt/cprocsp/bin/amd64/csptest",
			"csp_container":"container-id",
			"cert_hash":"hash",
			"http_listen_addr":":8000",
			"esia_response_cert_path":"TESIA GOST 2012.cer",
			"id_token_algorithm":"GOST3410_2012_256",
			"id_token_issuer":"https://esia.example.test/"
		}`)

		config, err := loadConfig(path)
		if err != nil {
			t.Fatalf("loadConfig() error = %v", err)
		}

		if config.Mnemonic != "TEST" {
			t.Fatalf("Mnemonic = %q, want %q", config.Mnemonic, "TEST")
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := loadConfig(filepath.Join(t.TempDir(), "missing.json"))
		if err == nil {
			t.Fatal("loadConfig() error = nil, want error")
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		path := writeTestConfig(t, `{"redirect_uri":`)

		_, err := loadConfig(path)
		if err == nil {
			t.Fatal("loadConfig() error = nil, want error")
		}
	})

	t.Run("empty required field", func(t *testing.T) {
		path := writeTestConfig(t, `{
			"redirect_uri":"https://example.test/callback",
			"mnemonic":"",
			"esia_uri":"https://esia.example.test",
			"csp_test_path":"/opt/cprocsp/bin/amd64/csptest",
			"csp_container":"container-id",
			"cert_hash":"hash",
			"http_listen_addr":":8000",
			"esia_response_cert_path":"TESIA GOST 2012.cer",
			"id_token_algorithm":"GOST3410_2012_256",
			"id_token_issuer":"https://esia.example.test/"
		}`)

		_, err := loadConfig(path)
		if err == nil {
			t.Fatal("loadConfig() error = nil, want error")
		}
		if !strings.Contains(err.Error(), "mnemonic") {
			t.Fatalf("loadConfig() error = %q, want field name mnemonic", err)
		}
	})

	t.Run("unsupported ID token algorithm", func(t *testing.T) {
		path := writeTestConfig(t, `{
			"redirect_uri":"https://example.test/callback",
			"mnemonic":"TEST",
			"esia_uri":"https://esia.example.test",
			"csp_test_path":"/opt/cprocsp/bin/amd64/csptest",
			"csp_container":"container-id",
			"cert_hash":"hash",
			"http_listen_addr":":8000",
			"esia_response_cert_path":"TESIA GOST 2012.cer",
			"id_token_algorithm":"none",
			"id_token_issuer":"https://esia.example.test/"
		}`)

		_, err := loadConfig(path)
		if err == nil {
			t.Fatal("loadConfig() error = nil, want error")
		}
		if !strings.Contains(err.Error(), "id_token_algorithm") {
			t.Fatalf("loadConfig() error = %q, want id_token_algorithm", err)
		}
	})
}

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	return path
}
