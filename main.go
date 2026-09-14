package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/ofstudio/go-api-epgu/esia/aas"
	"github.com/ofstudio/go-api-epgu/esia/signature"
)

const (
	preAuthCookieName = "__Host-esia_pre_auth"
	stateTTL          = 10 * time.Minute
)

type pendingLogin struct {
	expiresAt time.Time
	browserID string
}

type stateStore struct {
	mu      sync.Mutex
	entries map[string]pendingLogin
}

func newStateStore() *stateStore {
	return &stateStore{
		entries: make(map[string]pendingLogin),
	}
}

func (s *stateStore) save(
	state string,
	login pendingLogin,
) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.entries[state]; exists {
		return false
	}

	s.entries[state] = login

	return true
}

func (s *stateStore) consume(
	state string,
	browserID string,
	now time.Time,
) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	login, exists := s.entries[state]
	if !exists {
		return false
	}

	if !now.Before(login.expiresAt) {
		delete(s.entries, state)
		return false
	}

	if login.browserID != browserID {
		return false
	}
	delete(s.entries, state)

	return true
}

func (s *stateStore) deleteExpired(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for state, login := range s.entries {
		if !now.Before(login.expiresAt) {
			delete(s.entries, state)
		}
	}
}

func (s *stateStore) runCleanup(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case now := <-ticker.C:
			s.deleteExpired(now)

		case <-ctx.Done():
			return
		}
	}
}

func stateFromAuthURI(authURI string) (string, bool) {
	parsedURI, err := url.Parse(authURI)
	if err != nil {
		return "", false
	}

	states, exists := parsedURI.Query()["state"]
	if !exists || len(states) != 1 {
		return "", false
	}

	state := states[0]
	if state == "" {
		return "", false
	}

	return state, true
}

// withoutNilPermissions removes the permissions value that go-api-epgu v0.5.0
// emits when no consent-platform permissions were requested. The value bnVsbA
// is base64url-encoded JSON null. ESIA expects this optional parameter to be
// absent for a plain OpenID authorization request.
func withoutNilPermissions(authURI string) (string, error) {
	parsedURI, err := url.Parse(authURI)
	if err != nil {
		return "", fmt.Errorf("parse authorization URI: %w", err)
	}

	query := parsedURI.Query()
	permissions, exists := query["permissions"]
	if !exists {
		return authURI, nil
	}
	if len(permissions) != 1 || permissions[0] != "bnVsbA" {
		return "", errors.New("authorization URI contains unexpected permissions")
	}

	query.Del("permissions")
	parsedURI.RawQuery = query.Encode()

	return parsedURI.String(), nil
}

func randomBrowserID() (string, error) {
	randomBytes := make([]byte, 32)

	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(randomBytes), nil
}

func validBrowserID(browserID string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(browserID)
	if err != nil {
		return false
	}

	return len(decoded) == 32
}

func getOrCreateBrowserID(r *http.Request) (string, error) {
	cookie, err := r.Cookie(preAuthCookieName)

	if err == nil && validBrowserID(cookie.Value) {
		return cookie.Value, nil
	}

	return randomBrowserID()
}

func setPreAuthCookie(
	w http.ResponseWriter,
	browserID string,
	expiresAt time.Time,
) {
	http.SetCookie(w, &http.Cookie{
		Name:     preAuthCookieName,
		Value:    browserID,
		Path:     "/",
		MaxAge:   int(stateTTL.Seconds()),
		Expires:  expiresAt,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func main() {
	configPath := flag.String("config", "config.json", "путь к файлу конфигурации")
	flag.Parse()

	config, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("не удалось загрузить конфигурацию: %v", err)
	}

	signer := signature.NewLocalCryptoPro(
		config.CSPTestPath,
		config.CSPContainer,
		config.CertHash,
	)

	esiaHTTPClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	oauthClient := aas.
		NewClient(config.ESIAURI, config.Mnemonic, signer).
		WithHTTPClient(esiaHTTPClient)

	idTokenSignatureVerifier := newESIAIDTokenSignatureVerifier(
		config.CSPTestPath,
		config.ESIAResponseCertPath,
	)
	idTokenValidator := newIDTokenValidator(
		idTokenSignatureVerifier,
		config.IDTokenAlgorithm,
		config.Mnemonic,
		config.IDTokenIssuer,
	)

	pendingStates := newStateStore()
	mux := http.NewServeMux()

	mux.HandleFunc("/auth/esia/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")

		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		currentBrowserID, err := getOrCreateBrowserID(r)
		if err != nil {
			log.Print("ошибка создания идентификатора браузера")
			http.Error(
				w,
				"не удалось начать авторизацию через ЕСИА",
				http.StatusInternalServerError,
			)
			return
		}

		uri, err := oauthClient.AuthURI("openid", config.RedirectURI, nil)
		if err != nil {
			log.Print("ошибка создания авторизационной ссылки")
			http.Error(
				w,
				"не удалось начать авторизацию через ЕСИА",
				http.StatusInternalServerError,
			)
			return
		}
		uri, err = withoutNilPermissions(uri)
		if err != nil {
			log.Print("ошибка нормализации авторизационной ссылки")
			http.Error(
				w,
				"не удалось начать авторизацию через ЕСИА",
				http.StatusInternalServerError,
			)
			return
		}

		loginState, ok := stateFromAuthURI(uri)
		if !ok {
			log.Print("в авторизационной ссылке отсутствует корректный state")
			http.Error(
				w,
				"не удалось начать авторизацию через ЕСИА",
				http.StatusInternalServerError,
			)
			return
		}

		expiresAt := time.Now().Add(stateTTL)

		saved := pendingStates.save(
			loginState,
			pendingLogin{
				browserID: currentBrowserID,
				expiresAt: expiresAt,
			},
		)
		if !saved {
			log.Print("не удалось сохранить state авторизации")
			http.Error(
				w,
				"не удалось начать авторизацию через ЕСИА",
				http.StatusInternalServerError,
			)
			return
		}
		setPreAuthCookie(w, currentBrowserID, expiresAt)
		http.Redirect(w, r, uri, http.StatusFound)
	})
	mux.HandleFunc("/auth/esia/callback", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")

		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		code, callbackState, err := oauthClient.ParseCallback(r.URL.Query())
		if err != nil {
			log.Print("ошибка обработки callback ЕСИА")
			http.Error(
				w,
				"некорректный ответ ЕСИА",
				http.StatusBadRequest,
			)
			return
		}

		cookie, err := r.Cookie(preAuthCookieName)
		if err != nil || !validBrowserID(cookie.Value) {
			log.Print("в callback отсутствует корректная pre-auth cookie")
			http.Error(
				w,
				"некорректный ответ ЕСИА",
				http.StatusBadRequest,
			)
			return
		}

		if !pendingStates.consume(
			callbackState,
			cookie.Value,
			time.Now(),
		) {
			log.Print("state callback не прошёл проверку")
			http.Error(
				w,
				"некорректный ответ ЕСИА",
				http.StatusBadRequest,
			)
			return
		}
		tokenResponse, err := oauthClient.TokenExchange(
			code,
			"openid",
			config.RedirectURI,
		)

		if err != nil {
			log.Print("ошибка обмена кода на токен")
			http.Error(
				w,
				"не удалось завершить авторизацию через ЕСИА",
				http.StatusInternalServerError,
			)
			return
		}
		if tokenResponse == nil ||
			tokenResponse.AccessToken == "" ||
			tokenResponse.IdToken == "" {
			log.Print("ответ ЕСИА не содержит необходимые токены")
			http.Error(
				w,
				"неполный ответ ЕСИА",
				http.StatusBadGateway,
			)
			return
		}

		header, _, err := idTokenValidator.Validate(
			tokenResponse.IdToken,
			time.Now(),
		)
		if err != nil {
			log.Printf("ID token ЕСИА не прошёл проверку: %v", err)
			http.Error(
				w,
				"некорректный ответ ЕСИА",
				http.StatusBadGateway,
			)
			return
		}

		log.Printf(
			"ID token ЕСИА проверен: alg=%q, kid=%q",
			header.Algorithm,
			header.KeyID,
		)

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if _, err = fmt.Fprint(
			w,
			"авторизация через ЕСИА завершена",
		); err != nil {
			log.Print("ошибка отправки HTTP-ответа")
		}

	})

	server := &http.Server{
		Addr:              config.HTTPListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      45 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)

	go func() {
		log.Printf("сервер запущен на %s", config.HTTPListenAddr)
		serverErrors <- server.ListenAndServe()
	}()

	stopContext, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	go pendingStates.runCleanup(stopContext)

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}

		return

	case <-stopContext.Done():
		log.Print("получен сигнал завершения сервера")
	}

	shutdownContext, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	if err := server.Shutdown(shutdownContext); err != nil {
		log.Printf("не удалось корректно остановить сервер: %v", err)

		if closeErr := server.Close(); closeErr != nil &&
			!errors.Is(closeErr, http.ErrServerClosed) {
			log.Printf(
				"не удалось принудительно остановить сервер: %v",
				closeErr,
			)
		}
	}

	log.Print("сервер остановлен")
}
