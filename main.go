package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
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

// Локально вход иногда открывают через 127.0.0.1, а callback зарегистрирован на localhost.
// Для cookie это разные хосты, поэтому сразу приводим вход к одному адресу.
func canonicalLocalLoginURL(requestHost, redirectURI string) string {
	callback, err := url.Parse(redirectURI)
	if err != nil || callback.Scheme != "http" || callback.Hostname() != "localhost" {
		return ""
	}
	host, _, err := net.SplitHostPort(requestHost)
	if err != nil {
		host = requestHost
	}
	if host != "127.0.0.1" && host != "::1" {
		return ""
	}
	callback.Path = "/auth/esia/login"
	callback.RawQuery = ""
	callback.Fragment = ""
	return callback.String()
}

// Старая версия библиотеки иногда добавляет permissions=bnVsbA, то есть JSON null.
// ЕСИА такой параметр не любит, поэтому для обычного OpenID просто убираем его.
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
	if len(permissions) != 1 {
		return "", errors.New("authorization URI contains unexpected permissions")
	}
	if permissions[0] != "bnVsbA" {
		return authURI, nil
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
	requestedScope := config.oauthScope()
	// Тут специально остается openid: fullname и email запрашиваются ниже внутри permissions.
	log.Printf("ESIA OAuth scopes configured: %q", config.OAuth.Scopes)

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
	resourceClient := newESIAResourceClient(config.ESIAURI, esiaHTTPClient)

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
		if canonical := canonicalLocalLoginURL(r.Host, config.RedirectURI); canonical != "" {
			http.Redirect(w, r, canonical, http.StatusFound)
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

		uri, err := oauthClient.AuthURI(requestedScope, config.RedirectURI, config.Consent.Permissions)
		if err != nil {
			log.Print("ошибка создания авторизационной ссылки")
			http.Error(
				w,
				"не удалось начать авторизацию через ЕСИА",
				http.StatusInternalServerError,
			)
			return
		}
		if parsedURI, parseErr := url.Parse(uri); parseErr == nil {
			log.Printf("ESIA authorization request scopes: %q", parsedURI.Query().Get("scope"))
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

		// state привязываем к cookie браузера, чтобы чужой callback нельзя было подсунуть вручную.
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
	esiaCallbackHandler := func(w http.ResponseWriter, r *http.Request) {
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
			log.Printf("в callback отсутствует корректная pre-auth cookie: host=%q", r.Host)
			http.Error(
				w,
				"Сессия входа не найдена. Откройте /auth/esia/login в том же браузере и повторите попытку.",
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
		tokenExchangeStarted := time.Now()
		tokenResponse, err := oauthClient.TokenExchange(
			code,
			requestedScope,
			config.RedirectURI,
		)

		if err != nil {
			// Не записываем code, client_secret или токены: err содержит только
			// диагностическую причину библиотеки/ЕСИА и безопасен для журнала.
			log.Printf(
				"ошибка обмена кода на токен: duration=%s err=%v",
				time.Since(tokenExchangeStarted).Round(time.Millisecond),
				err,
			)
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

		header, claims, err := idTokenValidator.Validate(
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

		requestedScopes := config.scopeSet()
		subjectOID, err := esiaSubjectOID(claims.Subject)
		if err != nil {
			log.Printf("в ID token отсутствует корректный OID для запросов данных: %v", err)
			http.Error(w, "некорректный ответ ЕСИА", http.StatusBadGateway)
			return
		}

		person := Person{}
		if requestedScopes.Has("fullname") {
			person, err = resourceClient.getPerson(
				r.Context(),
				subjectOID,
				tokenResponse.AccessToken,
			)
			if err != nil {
				log.Printf("не удалось получить профиль ЕСИА: %v", err)
				http.Error(w, "не удалось получить профиль ЕСИА", http.StatusBadGateway)
				return
			}
		}

		contacts := Contacts{}
		if requestedScopes.Has("email") {
			contacts, err = resourceClient.getContacts(
				r.Context(),
				subjectOID,
				tokenResponse.AccessToken,
			)
			if err != nil {
				log.Printf("не удалось получить контакты ЕСИА: %v", err)
				http.Error(w, "не удалось получить контакты ЕСИА", http.StatusBadGateway)
				return
			}
		}

		fullName := strings.TrimSpace(strings.Join(
			[]string{person.LastName, person.FirstName, person.MiddleName},
			" ",
		))
		email := contacts.email()

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if _, err = fmt.Fprint(
			w,
			"авторизация через ЕСИА завершена\n",
			"ФИО: ", fullName, "\n",
			"Email: ", email,
		); err != nil {
			log.Print("ошибка отправки HTTP-ответа")
		}

	}
	mux.HandleFunc("/auth/esia/callback", esiaCallbackHandler)
	mux.HandleFunc("/callback", esiaCallbackHandler)

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
