package main

import (
	"net/url"
	"reflect"
	"testing"
)

func TestWithoutNilPermissions(t *testing.T) {
	t.Run("removes encoded JSON null and preserves OAuth parameters", func(t *testing.T) {
		const authURI = "https://esia.example.test/aas/oauth2/v2/ac?client_id=DEMO_CLIENT&permissions=bnVsbA&redirect_uri=https%3A%2F%2Fexample.test%2Fcallback&scope=openid&state=state-value"

		got, err := withoutNilPermissions(authURI)
		if err != nil {
			t.Fatalf("withoutNilPermissions() error = %v", err)
		}

		parsed, err := url.Parse(got)
		if err != nil {
			t.Fatalf("url.Parse() error = %v", err)
		}
		if _, exists := parsed.Query()["permissions"]; exists {
			t.Fatalf("permissions remains in URI: %q", got)
		}

		want := url.Values{
			"client_id":    {"DEMO_CLIENT"},
			"redirect_uri": {"https://example.test/callback"},
			"scope":        {"openid"},
			"state":        {"state-value"},
		}
		if !reflect.DeepEqual(parsed.Query(), want) {
			t.Fatalf("query = %#v, want %#v", parsed.Query(), want)
		}
	})

	t.Run("leaves URI without permissions unchanged", func(t *testing.T) {
		const authURI = "https://esia.example.test/aas/oauth2/v2/ac?scope=openid&state=state-value"

		got, err := withoutNilPermissions(authURI)
		if err != nil {
			t.Fatalf("withoutNilPermissions() error = %v", err)
		}
		if got != authURI {
			t.Fatalf("withoutNilPermissions() = %q, want unchanged URI", got)
		}
	})

	t.Run("rejects unexpected permissions", func(t *testing.T) {
		const authURI = "https://esia.example.test/aas/oauth2/v2/ac?permissions=eyJ1c2VkIjp0cnVlfQ"

		if _, err := withoutNilPermissions(authURI); err == nil {
			t.Fatal("withoutNilPermissions() error = nil, want error")
		}
	})

	t.Run("rejects malformed URI", func(t *testing.T) {
		if _, err := withoutNilPermissions("https://esia.example.test/%zz"); err == nil {
			t.Fatal("withoutNilPermissions() error = nil, want error")
		}
	})
}
