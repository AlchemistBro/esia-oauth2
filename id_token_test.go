package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type fakeIDTokenSignatureVerifier struct {
	verifyError error
	algorithm   string
	signedData  []byte
	signature   []byte
	called      bool
}

func (v *fakeIDTokenSignatureVerifier) Verify(
	algorithm string,
	signedData []byte,
	signature []byte,
) error {
	v.called = true
	v.algorithm = algorithm
	v.signedData = append([]byte(nil), signedData...)
	v.signature = append([]byte(nil), signature...)
	return v.verifyError
}

func TestIDTokenValidator(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	baseClaims := map[string]any{
		"aud": "DEMO_CLIENT",
		"sub": 1000000001,
		"iss": "https://esia.example.test/",
		"nbf": now.Add(-time.Minute).Unix(),
		"iat": now.Add(-time.Minute).Unix(),
		"exp": now.Add(10 * time.Minute).Unix(),
	}

	t.Run("valid token", func(t *testing.T) {
		verifier := &fakeIDTokenSignatureVerifier{}
		validator := newIDTokenValidator(
			verifier,
			idTokenAlgorithmGOST2012256,
			"DEMO_CLIENT",
			"https://esia.example.test/",
		)

		rawToken := testJWT(
			t,
			map[string]any{
				"alg": idTokenAlgorithmGOST2012256,
				"sbt": "id",
				"kid": "key-id",
				"typ": "JWT",
			},
			baseClaims,
			[]byte{1, 2, 3},
		)

		header, claims, err := validator.Validate(rawToken, now)
		if err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
		if header.Algorithm != idTokenAlgorithmGOST2012256 {
			t.Fatalf("Algorithm = %q", header.Algorithm)
		}
		if len(claims.Subject) == 0 {
			t.Fatal("Subject is empty")
		}
		if verifier.algorithm != idTokenAlgorithmGOST2012256 {
			t.Fatalf("verified algorithm = %q", verifier.algorithm)
		}
		if string(verifier.signedData) != strings.Join(strings.Split(rawToken, ".")[:2], ".") {
			t.Fatalf("unexpected signed data = %q", verifier.signedData)
		}
	})

	t.Run("rejects invalid sbt before signature verification", func(t *testing.T) {
		tests := []struct {
			name   string
			header map[string]any
		}{
			{
				name: "missing sbt",
				header: map[string]any{
					"alg": idTokenAlgorithmGOST2012256,
				},
			},
			{
				name: "access token sbt",
				header: map[string]any{
					"alg": idTokenAlgorithmGOST2012256,
					"sbt": "access",
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				verifier := &fakeIDTokenSignatureVerifier{}
				validator := newIDTokenValidator(
					verifier,
					idTokenAlgorithmGOST2012256,
					"DEMO_CLIENT",
					"https://esia.example.test/",
				)
				rawToken := testJWT(t, tt.header, baseClaims, []byte{1, 2, 3})

				_, _, err := validator.Validate(rawToken, now)
				if err == nil || !strings.Contains(err.Error(), "sbt") {
					t.Fatalf("Validate() error = %v, want sbt error", err)
				}
				if verifier.called {
					t.Fatal("signature verifier called for invalid sbt")
				}
			})
		}
	})

	tests := []struct {
		name      string
		header    map[string]any
		mutate    func(map[string]any)
		verifyErr error
		wantPart  string
	}{
		{
			name: "wrong algorithm",
			header: map[string]any{
				"alg": idTokenAlgorithmRS256,
				"sbt": "id",
			},
			wantPart: "неожиданный alg",
		},
		{
			name: "wrong issuer",
			header: map[string]any{
				"alg": idTokenAlgorithmGOST2012256,
				"sbt": "id",
			},
			mutate: func(claims map[string]any) {
				claims["iss"] = "http://attacker.example/"
			},
			wantPart: "некорректный iss",
		},
		{
			name: "wrong audience",
			header: map[string]any{
				"alg": idTokenAlgorithmGOST2012256,
				"sbt": "id",
			},
			mutate: func(claims map[string]any) {
				claims["aud"] = "OTHER"
			},
			wantPart: "другой аудитории",
		},
		{
			name: "expired",
			header: map[string]any{
				"alg": idTokenAlgorithmGOST2012256,
				"sbt": "id",
			},
			mutate: func(claims map[string]any) {
				claims["exp"] = now.Add(-2 * time.Minute).Unix()
			},
			wantPart: "истёк",
		},
		{
			name: "not before in future",
			header: map[string]any{
				"alg": idTokenAlgorithmGOST2012256,
				"sbt": "id",
			},
			mutate: func(claims map[string]any) {
				claims["nbf"] = now.Add(2 * time.Minute).Unix()
			},
			wantPart: "ещё не вступил",
		},
		{
			name: "issued at in future",
			header: map[string]any{
				"alg": idTokenAlgorithmGOST2012256,
				"sbt": "id",
			},
			mutate: func(claims map[string]any) {
				claims["iat"] = now.Add(2 * time.Minute).Unix()
			},
			wantPart: "iat",
		},
		{
			name: "signature rejected",
			header: map[string]any{
				"alg": idTokenAlgorithmGOST2012256,
				"sbt": "id",
			},
			verifyErr: errors.New("bad signature"),
			wantPart:  "bad signature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := cloneClaims(baseClaims)
			if tt.mutate != nil {
				tt.mutate(claims)
			}

			verifier := &fakeIDTokenSignatureVerifier{verifyError: tt.verifyErr}
			validator := newIDTokenValidator(
				verifier,
				idTokenAlgorithmGOST2012256,
				"DEMO_CLIENT",
				"https://esia.example.test/",
			)

			rawToken := testJWT(t, tt.header, claims, []byte{1, 2, 3})
			_, _, err := validator.Validate(rawToken, now)
			if err == nil {
				t.Fatal("Validate() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantPart) {
				t.Fatalf("Validate() error = %q, want %q", err, tt.wantPart)
			}
		})
	}
}

type inspectingRunner struct {
	t                 *testing.T
	wantSignedData    []byte
	wantCryptoProSign []byte
	called            bool
}

func (r *inspectingRunner) Run(name string, args ...string) ([]byte, error) {
	r.t.Helper()
	r.called = true

	if name != "/opt/cprocsp/bin/amd64/csptest" {
		r.t.Fatalf("command = %q", name)
	}

	wantPrefix := []string{"-keyset", "-verify", "GOST12_256", "-in"}
	if len(args) < 9 || !reflect.DeepEqual(args[:4], wantPrefix) {
		r.t.Fatalf("args = %#v", args)
	}
	if args[5] != "-signature" || args[7] != "-certificate" || args[8] != "TESIA GOST 2012.cer" {
		r.t.Fatalf("args = %#v", args)
	}

	data, err := os.ReadFile(args[4])
	if err != nil {
		r.t.Fatalf("ReadFile(data) error = %v", err)
	}
	if !reflect.DeepEqual(data, r.wantSignedData) {
		r.t.Fatalf("signed data = %x", data)
	}

	signature, err := os.ReadFile(args[6])
	if err != nil {
		r.t.Fatalf("ReadFile(signature) error = %v", err)
	}
	if !reflect.DeepEqual(signature, r.wantCryptoProSign) {
		r.t.Fatalf("signature = %x, want %x", signature, r.wantCryptoProSign)
	}

	return []byte("OK"), nil
}

func TestGOSTVerifierReversesJWTSignatureForCryptoPro(t *testing.T) {
	signature := make([]byte, 64)
	for i := range signature {
		signature[i] = byte(i)
	}

	wantReversed := append([]byte(nil), signature...)
	for left, right := 0, len(wantReversed)-1; left < right; left, right = left+1, right-1 {
		wantReversed[left], wantReversed[right] = wantReversed[right], wantReversed[left]
	}

	runner := &inspectingRunner{
		t:                 t,
		wantSignedData:    []byte("header.payload"),
		wantCryptoProSign: wantReversed,
	}
	verifier := &esiaIDTokenSignatureVerifier{
		cspTestPath: "/opt/cprocsp/bin/amd64/csptest",
		certPath:    "TESIA GOST 2012.cer",
		runner:      runner,
	}

	if err := verifier.Verify(
		idTokenAlgorithmGOST2012256,
		[]byte("header.payload"),
		signature,
	); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if !runner.called {
		t.Fatal("CryptoPro command was not called")
	}
}

func TestRS256Verifier(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
	}

	now := time.Now()
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test ESIA"},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(
		rand.Reader,
		&template,
		&template,
		&privateKey.PublicKey,
		privateKey,
	)
	if err != nil {
		t.Fatalf("x509.CreateCertificate() error = %v", err)
	}

	certPath := filepath.Join(t.TempDir(), "esia.cer")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	signedData := []byte("header.payload")
	digest := sha256.Sum256(signedData)
	signature, err := rsa.SignPKCS1v15(
		rand.Reader,
		privateKey,
		crypto.SHA256,
		digest[:],
	)
	if err != nil {
		t.Fatalf("rsa.SignPKCS1v15() error = %v", err)
	}

	verifier := newESIAIDTokenSignatureVerifier("unused", certPath)
	if err := verifier.Verify(idTokenAlgorithmRS256, signedData, signature); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}

	signature[0] ^= 0xff
	if err := verifier.Verify(idTokenAlgorithmRS256, signedData, signature); err == nil {
		t.Fatal("Verify() error = nil for corrupted signature")
	}
}

func testJWT(
	t *testing.T,
	header map[string]any,
	claims map[string]any,
	signature []byte,
) string {
	t.Helper()

	headerJSON, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("json.Marshal(header) error = %v", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("json.Marshal(claims) error = %v", err)
	}

	return base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(claimsJSON) + "." +
		base64.RawURLEncoding.EncodeToString(signature)
}

func cloneClaims(source map[string]any) map[string]any {
	copy := make(map[string]any, len(source))
	for key, value := range source {
		copy[key] = value
	}
	return copy
}
