package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ofstudio/go-api-epgu/esia/aas"
)

var supportedESIAScopes = map[string]struct{}{
	"openid":   {},
	"fullname": {},
	"email":    {},
}

type OAuthConfig struct {
	Scopes []string `json:"scopes"`
}

type ConsentConfig struct {
	Permissions aas.Permissions `json:"permissions"`
}

type ScopeSet map[string]struct{}

func (s ScopeSet) Has(scope string) bool {
	_, exists := s[scope]
	return exists
}

type Config struct {
	RedirectURI          string        `json:"redirect_uri"`
	Mnemonic             string        `json:"mnemonic"`
	ESIAURI              string        `json:"esia_uri"`
	CSPTestPath          string        `json:"csp_test_path"`
	CSPContainer         string        `json:"csp_container"`
	CertHash             string        `json:"cert_hash"`
	HTTPListenAddr       string        `json:"http_listen_addr"`
	ESIAResponseCertPath string        `json:"esia_response_cert_path"`
	IDTokenAlgorithm     string        `json:"id_token_algorithm"`
	IDTokenIssuer        string        `json:"id_token_issuer"`
	OAuth                OAuthConfig   `json:"oauth"`
	Consent              ConsentConfig `json:"consent"`
}

func (c Config) oauthScope() string {
	return strings.Join(c.OAuth.Scopes, " ")
}

func (c Config) scopeSet() ScopeSet {
	scopes := make(ScopeSet, len(c.OAuth.Scopes))
	for _, scope := range c.OAuth.Scopes {
		scopes[scope] = struct{}{}
	}
	for _, permission := range c.Consent.Permissions {
		for _, scope := range permission.Scopes {
			scopes[scope.Sysname] = struct{}{}
		}
	}
	return scopes
}

func loadConfig(path string) (Config, error) {
	var config Config

	data, err := os.ReadFile(path)
	if err != nil {
		return config, fmt.Errorf("не удалось прочитать файл %q: %w", path, err)
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("некорректный JSON в файле %q: %w", path, err)
	}

	if err := config.validate(); err != nil {
		return config, fmt.Errorf("некорректная конфигурация в файле %q: %w", path, err)
	}

	return config, nil
}

func (c Config) validate() error {
	required := []struct {
		name  string
		value string
	}{
		{name: "redirect_uri", value: c.RedirectURI},
		{name: "mnemonic", value: c.Mnemonic},
		{name: "esia_uri", value: c.ESIAURI},
		{name: "csp_test_path", value: c.CSPTestPath},
		{name: "csp_container", value: c.CSPContainer},
		{name: "cert_hash", value: c.CertHash},
		{name: "http_listen_addr", value: c.HTTPListenAddr},
		{name: "esia_response_cert_path", value: c.ESIAResponseCertPath},
		{name: "id_token_algorithm", value: c.IDTokenAlgorithm},
		{name: "id_token_issuer", value: c.IDTokenIssuer},
	}

	for _, field := range required {
		if strings.TrimSpace(field.value) == "" {
			return errors.New("обязательное поле " + field.name + " не задано")
		}
	}

	switch c.IDTokenAlgorithm {
	case idTokenAlgorithmGOST2012256, idTokenAlgorithmRS256:
	default:
		return fmt.Errorf(
			"поле id_token_algorithm содержит неподдерживаемое значение %q",
			c.IDTokenAlgorithm,
		)
	}

	if len(c.OAuth.Scopes) == 0 {
		return errors.New("oauth.scopes не должен быть пустым")
	}

	seenScopes := make(map[string]struct{}, len(c.OAuth.Scopes))
	for _, scope := range c.OAuth.Scopes {
		if strings.TrimSpace(scope) == "" {
			return errors.New("oauth.scopes не должен содержать пустые значения")
		}
		if _, supported := supportedESIAScopes[scope]; !supported {
			return fmt.Errorf("неподдерживаемый ESIA scope %q", scope)
		}
		if _, exists := seenScopes[scope]; exists {
			return fmt.Errorf("oauth.scopes содержит повторяющийся scope %q", scope)
		}
		seenScopes[scope] = struct{}{}
	}
	if _, exists := seenScopes["openid"]; !exists {
		return errors.New("oauth.scopes должен содержать openid")
	}
	if len(c.Consent.Permissions) > 0 {
		if len(c.OAuth.Scopes) != 1 || c.OAuth.Scopes[0] != "openid" {
			return errors.New("при consent.permissions oauth.scopes должен содержать только openid")
		}
		for permissionIndex, permission := range c.Consent.Permissions {
			prefix := fmt.Sprintf("consent.permissions[%d]", permissionIndex)
			if strings.TrimSpace(permission.Sysname) == "" {
				return errors.New(prefix + ".sysname не задан")
			}
			if permission.Expire <= 0 {
				return errors.New(prefix + ".expire должен быть больше нуля")
			}
			if len(permission.Actions) == 0 || len(permission.Purposes) == 0 || len(permission.Scopes) == 0 {
				return errors.New(prefix + " должен содержать actions, purposes и scopes")
			}
			for _, action := range permission.Actions {
				if strings.TrimSpace(action.Sysname) == "" {
					return errors.New(prefix + ".actions содержит пустой sysname")
				}
			}
			for _, purpose := range permission.Purposes {
				if strings.TrimSpace(purpose.Sysname) == "" {
					return errors.New(prefix + ".purposes содержит пустой sysname")
				}
			}
			for _, scope := range permission.Scopes {
				if strings.TrimSpace(scope.Sysname) == "" {
					return errors.New(prefix + ".scopes содержит пустой sysname")
				}
			}
		}
	}

	return nil
}
