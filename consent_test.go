package main

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/ofstudio/go-api-epgu/esia/aas"
)

type consentTestSigner struct{}

func (consentTestSigner) Sign([]byte) ([]byte, error) { return []byte("test-signature"), nil }
func (consentTestSigner) CertHash() string            { return "test-hash" }

func TestFormalConsentAuthorizationURI(t *testing.T) {
	config := Config{
		OAuth: OAuthConfig{Scopes: []string{"openid"}},
		Consent: ConsentConfig{Permissions: aas.Permissions{{
			Sysname:  "APIPGU",
			Expire:   525600,
			Actions:  []aas.PermissionAction{{Sysname: "ALL_ACTIONS_TO_DATA"}},
			Purposes: []aas.PermissionPurpose{{Sysname: "APIPGU"}},
			Scopes: []aas.PermissionScope{
				{Sysname: "fullname"},
				{Sysname: "email"},
			},
		}}},
	}

	client := aas.NewClient("https://esia.example.test", "TEST", consentTestSigner{})
	uri, err := client.AuthURI(config.oauthScope(), "https://example.test/callback", config.Consent.Permissions)
	if err != nil {
		t.Fatalf("AuthURI() error = %v", err)
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	if got := parsed.Query().Get("scope"); got != "openid" {
		t.Fatalf("scope = %q, want openid", got)
	}
	encoded := parsed.Query().Get("permissions")
	if encoded == "" || encoded == "bnVsbA" {
		t.Fatalf("permissions = %q, want encoded consent", encoded)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode permissions: %v", err)
	}
	var got aas.Permissions
	if err := json.Unmarshal(decoded, &got); err != nil {
		t.Fatalf("decode permissions JSON: %v", err)
	}
	if !reflect.DeepEqual(got, config.Consent.Permissions) {
		t.Fatalf("permissions = %#v, want %#v", got, config.Consent.Permissions)
	}
	if strings.Contains(string(decoded), "api-order") {
		t.Fatalf("unexpected api-order in permissions")
	}
	if got := config.scopeSet(); !got.Has("fullname") || !got.Has("email") {
		t.Fatalf("resource scopes missing: %#v", got)
	}
}

func TestFormalConsentRejectsMixedOAuthScopes(t *testing.T) {
	config, err := loadConfig(writeTestConfig(t, validTestConfig(`["openid","fullname"]`)))
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	config.Consent = ConsentConfig{Permissions: aas.Permissions{{
		Sysname:  "APIPGU",
		Expire:   1,
		Actions:  []aas.PermissionAction{{Sysname: "ALL_ACTIONS_TO_DATA"}},
		Purposes: []aas.PermissionPurpose{{Sysname: "APIPGU"}},
		Scopes:   []aas.PermissionScope{{Sysname: "fullname"}},
	}}}
	if err := config.validate(); err == nil || !strings.Contains(err.Error(), "только openid") {
		t.Fatalf("validate() error = %v, want only-openid error", err)
	}
}
