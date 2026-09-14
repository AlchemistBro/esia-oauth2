package main

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const idTokenClockSkew = 60 * time.Second

const (
	idTokenAlgorithmGOST2012256 = "GOST3410_2012_256"
	idTokenAlgorithmRS256       = "RS256"
)

type idTokenHeader struct {
	Algorithm string `json:"alg"`
	SBT       string `json:"sbt"`
	KeyID     string `json:"kid"`
	Type      string `json:"typ"`
}

type idTokenClaims struct {
	Audience  audienceClaim   `json:"aud"`
	ExpiresAt int64           `json:"exp"`
	IssuedAt  int64           `json:"iat"`
	Issuer    string          `json:"iss"`
	NotBefore int64           `json:"nbf"`
	Subject   json.RawMessage `json:"sub"`
}

type audienceClaim []string

func (a *audienceClaim) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*a = audienceClaim{single}
		return nil
	}

	var multiple []string
	if err := json.Unmarshal(data, &multiple); err != nil {
		return errors.New("claim aud должен быть строкой или массивом строк")
	}

	*a = audienceClaim(multiple)
	return nil
}

func (a audienceClaim) contains(value string) bool {
	for _, candidate := range a {
		if candidate == value {
			return true
		}
	}

	return false
}

type idTokenSignatureVerifier interface {
	Verify(
		algorithm string,
		signedData []byte,
		signature []byte,
	) error
}

type commandRunner interface {
	Run(name string, args ...string) ([]byte, error)
}

type execCommandRunner struct{}

func (execCommandRunner) Run(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

type esiaIDTokenSignatureVerifier struct {
	cspTestPath string
	certPath    string
	runner      commandRunner
}

func newESIAIDTokenSignatureVerifier(
	cspTestPath string,
	certPath string,
) *esiaIDTokenSignatureVerifier {
	return &esiaIDTokenSignatureVerifier{
		cspTestPath: cspTestPath,
		certPath:    certPath,
		runner:      execCommandRunner{},
	}
}

func (v *esiaIDTokenSignatureVerifier) Verify(
	algorithm string,
	signedData []byte,
	signature []byte,
) error {
	switch algorithm {
	case idTokenAlgorithmGOST2012256:
		return v.verifyGOST2012256(signedData, signature)

	case idTokenAlgorithmRS256:
		return v.verifyRS256(signedData, signature)

	default:
		return fmt.Errorf(
			"неподдерживаемый алгоритм подписи ID token: %q",
			algorithm,
		)
	}
}

func (v *esiaIDTokenSignatureVerifier) verifyGOST2012256(
	signedData []byte,
	signature []byte,
) error {
	if len(signature) != 64 {
		return fmt.Errorf(
			"некорректная длина ГОСТ-подписи ID token: %d байт",
			len(signature),
		)
	}

	if strings.TrimSpace(v.cspTestPath) == "" {
		return errors.New("не задан путь к csptest для проверки ID token")
	}
	if strings.TrimSpace(v.certPath) == "" {
		return errors.New("не задан путь к сертификату ЕСИА для проверки ID token")
	}

	tempDir, err := os.MkdirTemp("", "esia-id-token-*")
	if err != nil {
		return fmt.Errorf(
			"не удалось создать временный каталог для проверки ID token: %w",
			err,
		)
	}
	defer os.RemoveAll(tempDir)

	dataPath := filepath.Join(tempDir, "signed-data.bin")
	signaturePath := filepath.Join(tempDir, "signature.bin")

	if err := os.WriteFile(dataPath, signedData, 0o600); err != nil {
		return fmt.Errorf(
			"не удалось подготовить данные ID token для проверки: %w",
			err,
		)
	}

	// ЕСИА кодирует RAW-подпись ГОСТ в JWT в big-endian представлении.
	// CryptoPro CSP ожидает обратный порядок байтов для RAW-подписи.
	cryptoProSignature := append([]byte(nil), signature...)
	for left, right := 0, len(cryptoProSignature)-1; left < right; left, right = left+1, right-1 {
		cryptoProSignature[left], cryptoProSignature[right] =
			cryptoProSignature[right], cryptoProSignature[left]
	}

	if err := os.WriteFile(signaturePath, cryptoProSignature, 0o600); err != nil {
		return fmt.Errorf(
			"не удалось подготовить подпись ID token для проверки: %w",
			err,
		)
	}

	output, err := v.runner.Run(
		v.cspTestPath,
		"-keyset",
		"-verify",
		"GOST12_256",
		"-in",
		dataPath,
		"-signature",
		signaturePath,
		"-certificate",
		v.certPath,
	)
	if err != nil {
		message := strings.TrimSpace(string(output))
		if len(message) > 2000 {
			message = message[:2000] + "..."
		}

		if message == "" {
			return fmt.Errorf("CryptoPro отклонил подпись ID token: %w", err)
		}

		return fmt.Errorf(
			"CryptoPro отклонил подпись ID token: %w: %s",
			err,
			message,
		)
	}

	return nil
}

func (v *esiaIDTokenSignatureVerifier) verifyRS256(
	signedData []byte,
	signature []byte,
) error {
	certificate, err := readX509Certificate(v.certPath)
	if err != nil {
		return err
	}

	publicKey, ok := certificate.PublicKey.(*rsa.PublicKey)
	if !ok {
		return errors.New("сертификат ЕСИА не содержит RSA-ключ")
	}

	now := time.Now()
	if now.Before(certificate.NotBefore) || now.After(certificate.NotAfter) {
		return errors.New("срок действия RSA-сертификата ЕСИА истёк или ещё не начался")
	}

	digest := sha256.Sum256(signedData)
	if err := rsa.VerifyPKCS1v15(
		publicKey,
		crypto.SHA256,
		digest[:],
		signature,
	); err != nil {
		return errors.New("RSA-подпись ID token не прошла проверку")
	}

	return nil
}

func readX509Certificate(path string) (*x509.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf(
			"не удалось прочитать сертификат ЕСИА %q: %w",
			path,
			err,
		)
	}

	if block, _ := pem.Decode(data); block != nil {
		if block.Type != "CERTIFICATE" {
			return nil, fmt.Errorf(
				"файл %q не содержит PEM-сертификат",
				path,
			)
		}
		data = block.Bytes
	}

	certificate, err := x509.ParseCertificate(data)
	if err != nil {
		return nil, fmt.Errorf(
			"не удалось разобрать RSA-сертификат ЕСИА %q: %w",
			path,
			err,
		)
	}

	return certificate, nil
}

type idTokenValidator struct {
	signatureVerifier idTokenSignatureVerifier
	expectedAlgorithm string
	expectedAudience  string
	expectedIssuer    string
}

func newIDTokenValidator(
	signatureVerifier idTokenSignatureVerifier,
	expectedAlgorithm string,
	expectedAudience string,
	expectedIssuer string,
) *idTokenValidator {
	return &idTokenValidator{
		signatureVerifier: signatureVerifier,
		expectedAlgorithm: expectedAlgorithm,
		expectedAudience:  expectedAudience,
		expectedIssuer:    expectedIssuer,
	}
}

func (v *idTokenValidator) Validate(
	rawToken string,
	now time.Time,
) (idTokenHeader, idTokenClaims, error) {
	var header idTokenHeader
	var claims idTokenClaims

	parts := strings.Split(rawToken, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return header, claims, errors.New("некорректный формат ID token")
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return header, claims, errors.New("не удалось декодировать заголовок ID token")
	}

	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return header, claims, errors.New("некорректный JSON заголовка ID token")
	}

	if header.Algorithm == "" {
		return header, claims, errors.New("в заголовке ID token отсутствует alg")
	}
	if header.Algorithm != v.expectedAlgorithm {
		return header, claims, fmt.Errorf(
			"неожиданный alg ID token: получено %q, ожидается %q",
			header.Algorithm,
			v.expectedAlgorithm,
		)
	}
	if header.Type != "" && header.Type != "JWT" {
		return header, claims, fmt.Errorf(
			"некорректный typ ID token: %q",
			header.Type,
		)
	}
	if header.SBT != "id" {
		return header, claims, fmt.Errorf(
			"некорректный sbt ID token: %q",
			header.SBT,
		)
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return header, claims, errors.New("не удалось декодировать payload ID token")
	}

	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return header, claims, errors.New("некорректный JSON payload ID token")
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return header, claims, errors.New("не удалось декодировать подпись ID token")
	}
	if len(signature) == 0 {
		return header, claims, errors.New("ID token не содержит подпись")
	}
	if v.signatureVerifier == nil {
		return header, claims, errors.New("не настроена проверка подписи ID token")
	}

	signedData := []byte(parts[0] + "." + parts[1])
	if err := v.signatureVerifier.Verify(
		header.Algorithm,
		signedData,
		signature,
	); err != nil {
		return header, claims, err
	}

	if claims.Issuer == "" {
		return header, claims, errors.New("в ID token отсутствует iss")
	}
	if claims.Issuer != v.expectedIssuer {
		return header, claims, fmt.Errorf(
			"некорректный iss ID token: получено %q",
			claims.Issuer,
		)
	}

	if len(claims.Audience) == 0 || !claims.Audience.contains(v.expectedAudience) {
		return header, claims, errors.New("ID token выдан другой аудитории")
	}

	if len(bytes.TrimSpace(claims.Subject)) == 0 || bytes.Equal(claims.Subject, []byte("null")) {
		return header, claims, errors.New("в ID token отсутствует sub")
	}

	if claims.ExpiresAt == 0 {
		return header, claims, errors.New("в ID token отсутствует exp")
	}
	if !now.Before(time.Unix(claims.ExpiresAt, 0).Add(idTokenClockSkew)) {
		return header, claims, errors.New("срок действия ID token истёк")
	}

	if claims.NotBefore == 0 {
		return header, claims, errors.New("в ID token отсутствует nbf")
	}
	if now.Add(idTokenClockSkew).Before(time.Unix(claims.NotBefore, 0)) {
		return header, claims, errors.New("ID token ещё не вступил в действие")
	}

	if claims.IssuedAt == 0 {
		return header, claims, errors.New("в ID token отсутствует iat")
	}
	if now.Add(idTokenClockSkew).Before(time.Unix(claims.IssuedAt, 0)) {
		return header, claims, errors.New("iat ID token находится в будущем")
	}

	if claims.NotBefore > claims.ExpiresAt {
		return header, claims, errors.New("nbf ID token находится после exp")
	}
	if claims.IssuedAt > claims.ExpiresAt {
		return header, claims, errors.New("iat ID token находится после exp")
	}

	return header, claims, nil
}
