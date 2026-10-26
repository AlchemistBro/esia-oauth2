package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/ofstudio/go-api-epgu/esia/aas"
)

func TestESIAOAuthFlow(t *testing.T) {
	for _, mismatch := range []bool{false, true} {
		name := "matching token response state"
		if mismatch {
			name = "rejects mismatched token response state"
		}
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != aas.TokenEndpoint {
					t.Errorf("unexpected token request: %s %s", r.Method, r.URL.Path)
				}
				if err := r.ParseForm(); err != nil {
					t.Errorf("parse token request: %v", err)
				}
				if r.Form.Get("code") != "test-code" || r.Form.Get("scope") != "openid" || r.Form.Get("redirect_uri") != "https://example.test/callback" {
					t.Error("token request lost authorization parameters")
				}
				state := r.Form.Get("state")
				if state == "" {
					t.Error("token request has no state")
				}
				if mismatch {
					state = "wrong-state"
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(aas.TokenExchangeResponse{State: state, AccessToken: "test-access", IdToken: "test-id"})
			}))
			defer server.Close()
			client := aas.NewClient(server.URL, "TEST", consentTestSigner{}).WithHTTPClient(server.Client())
			uri, state, err := client.AuthURI("openid", "https://example.test/callback", nil)
			if err != nil {
				t.Fatal(err)
			}
			uri, err = withoutNilPermissions(uri)
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := url.Parse(uri)
			if err != nil {
				t.Fatal(err)
			}
			if state == "" || parsed.Query().Get("state") != state || parsed.Query().Has("permissions") {
				t.Fatal("normalization must preserve authorization state and omit nil permissions")
			}
			store := newStateStore()
			store.save(state, pendingLogin{browserID: "browser", expiresAt: time.Now().Add(stateTTL)})
			code, callbackState, err := client.ParseCallback(url.Values{"code": {"test-code"}, "state": {state}})
			if err != nil || !store.consume(callbackState, "browser", time.Now()) {
				t.Fatalf("callback did not match stored authorization state: %v", err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			response, err := client.TokenExchange(ctx, code, "openid", "https://example.test/callback")
			if mismatch {
				if !errors.Is(err, aas.ErrStateMismatch) {
					t.Fatalf("TokenExchange() error = %v, want state mismatch", err)
				}
			} else if err != nil || response == nil || response.AccessToken != "test-access" || response.IdToken != "test-id" {
				t.Fatalf("TokenExchange() = %#v, %v", response, err)
			}
			cancel()
			if _, err := client.TokenExchange(ctx, code, "openid", "https://example.test/callback"); !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled TokenExchange() error = %v", err)
			}
		})
	}
}
