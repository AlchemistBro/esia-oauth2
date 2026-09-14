package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	RedirectURI          string `json:"redirect_uri"`
	Mnemonic             string `json:"mnemonic"`
	ESIAURI              string `json:"esia_uri"`
	CSPTestPath          string `json:"csp_test_path"`
	CSPContainer         string `json:"csp_container"`
	CertHash             string `json:"cert_hash"`
	HTTPListenAddr       string `json:"http_listen_addr"`
	ESIAResponseCertPath string `json:"esia_response_cert_path"`
	IDTokenAlgorithm     string `json:"id_token_algorithm"`
	IDTokenIssuer        string `json:"id_token_issuer"`
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

	return nil
}
