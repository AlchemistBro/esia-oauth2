package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestESIAResourceClientGetPerson(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/rs/prns/1000000001"; got != want {
			t.Fatalf("path = %q, want %q", got, want)
		}
		if got, want := r.Header.Get("Authorization"), "Bearer test-access-token"; got != want {
			t.Fatalf("Authorization = %q, want %q", got, want)
		}
		if got, want := r.Header.Get("Accept"), "application/json"; got != want {
			t.Fatalf("Accept = %q, want %q", got, want)
		}
		_, _ = w.Write([]byte(`{"firstName":"Test","lastName":"User"}`))
	}))
	defer server.Close()

	client := newESIAResourceClient(server.URL, server.Client())
	person, err := client.getPerson(context.Background(), "1000000001", "test-access-token")
	if err != nil {
		t.Fatalf("getPerson() error = %v", err)
	}
	if !person.fullNamePresent() {
		t.Fatal("fullNamePresent() = false, want true")
	}
}

func TestESIASubjectOID(t *testing.T) {
	tests := []struct {
		name    string
		subject string
		want    string
		wantErr bool
	}{
		{name: "numeric JSON", subject: `1000000001`, want: "1000000001"},
		{name: "string JSON", subject: `"1000000001"`, want: "1000000001"},
		{name: "not an OID", subject: `"not-an-oid"`, wantErr: true},
		{name: "object", subject: `{}`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := esiaSubjectOID([]byte(tt.subject))
			if tt.wantErr {
				if err == nil {
					t.Fatal("esiaSubjectOID() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("esiaSubjectOID() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("esiaSubjectOID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestESIAResourceClientGetContacts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/rs/prns/1000000001/ctts"; got != want {
			t.Fatalf("path = %q, want %q", got, want)
		}
		if got, want := r.URL.Query().Get("embed"), "(elements)"; got != want {
			t.Fatalf("embed = %q, want %q", got, want)
		}
		if got, want := r.Header.Get("Authorization"), "Bearer test-access-token"; got != want {
			t.Fatalf("Authorization = %q, want %q", got, want)
		}
		_, _ = w.Write([]byte(`{"elements":[{"id":1,"type":"EML","value":"not-logged@example.test","vrfStu":"VERIFIED"}]}`))
	}))
	defer server.Close()

	client := newESIAResourceClient(server.URL, server.Client())
	contacts, err := client.getContacts(context.Background(), "1000000001", "test-access-token")
	if err != nil {
		t.Fatalf("getContacts() error = %v", err)
	}
	if got, want := contacts.types(), []string{"EML"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("types() = %#v, want %#v", got, want)
	}
	if !contacts.emailPresent() {
		t.Fatal("emailPresent() = false, want true")
	}
}

func TestESIAResourceClientRejectsBadResponses(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "non success status", status: http.StatusBadGateway, body: `{"error":"upstream unavailable"}`},
		{name: "malformed JSON", status: http.StatusOK, body: `{`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := newESIAResourceClient(server.URL, server.Client())
			if _, err := client.getContacts(context.Background(), "1000000001", "test-access-token"); err == nil {
				t.Fatal("getContacts() error = nil, want error")
			}
		})
	}
}
