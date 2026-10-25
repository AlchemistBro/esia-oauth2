package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	t.Run("safe fixture config", func(t *testing.T) {
		path := writeTestConfig(t, `{
			"redirect_uri":"https://example.test/callback",
			"mnemonic":"DEMO_CLIENT",
			"esia_uri":"https://esia.example.test",
			"csp_test_path":"/opt/cprocsp/bin/amd64/csptest",
			"csp_container":"demo-container",
			"cert_hash":"demo-fingerprint",
			"http_listen_addr":":8000",
			"esia_response_cert_path":"/path/to/tesia-gost.cer",
			"id_token_algorithm":"GOST3410_2012_256",
			"id_token_issuer":"http://esia.example.test/",
			"oauth":{"scopes":["openid","fullname","email"]}
		}`)
		if _, err := loadConfig(path); err != nil {
			t.Fatalf("loadConfig(safe fixture) error = %v", err)
		}
	})

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
			"id_token_issuer":"http://esia-portal1.test.gosuslugi.ru/",
			"oauth":{"scopes":["openid","fullname","email"]}
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
			"id_token_issuer":"http://esia-portal1.test.gosuslugi.ru/",
			"oauth":{"scopes":["openid","fullname","email"]}
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
			"id_token_issuer":"http://esia-portal1.test.gosuslugi.ru/",
			"oauth":{"scopes":["openid","fullname","email"]}
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

func TestOAuthScopesValidation(t *testing.T) {
	tests := []struct {
		name      string
		scopes    string
		wantErr   string
		wantScope string
	}{
		{name: "openid only", scopes: `["openid"]`, wantScope: "openid"},
		{name: "openid and fullname", scopes: `["openid","fullname"]`, wantScope: "openid fullname"},
		{name: "all supported scopes", scopes: `["openid","fullname","email"]`, wantScope: "openid fullname email"},
		{name: "empty", scopes: `[]`, wantErr: "oauth.scopes не должен быть пустым"},
		{name: "openid missing", scopes: `["fullname"]`, wantErr: "должен содержать openid"},
		{name: "empty value", scopes: `["openid",""]`, wantErr: "пустые значения"},
		{name: "unknown value", scopes: `["openid","profile"]`, wantErr: "неподдерживаемый ESIA scope"},
		{name: "duplicate", scopes: `["openid","openid"]`, wantErr: "повторяющийся scope"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTestConfig(t, validTestConfig(tt.scopes))
			config, err := loadConfig(path)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("loadConfig() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("loadConfig() error = %v", err)
			}
			if got := config.oauthScope(); got != tt.wantScope {
				t.Fatalf("oauthScope() = %q, want %q", got, tt.wantScope)
			}
		})
	}
}

func validTestConfig(scopes string) string {
	return `{
		"redirect_uri":"https://example.test/callback",
		"mnemonic":"TEST",
		"esia_uri":"https://esia.example.test",
		"csp_test_path":"/opt/cprocsp/bin/amd64/csptest",
		"csp_container":"container-id",
		"cert_hash":"hash",
		"http_listen_addr":":8000",
		"esia_response_cert_path":"TESIA GOST 2012.cer",
		"id_token_algorithm":"GOST3410_2012_256",
		"id_token_issuer":"http://esia.example.test/",
		"oauth":{"scopes":` + scopes + `}
	}`
}

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	return path
}

func TestExampleConfigLoads(t *testing.T) {
	if _, err := loadConfig("config.example.json"); err != nil {
		t.Fatalf("config.example.json must stay valid: %v", err)
	}
}
