package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const maxESIAResourceErrorBody = 4 << 10

type Person struct {
	FirstName  string `json:"firstName"`
	LastName   string `json:"lastName"`
	MiddleName string `json:"middleName"`
}

func (p Person) fullNamePresent() bool {
	return p.FirstName != "" || p.LastName != "" || p.MiddleName != ""
}

type Contacts struct {
	Elements []Contact `json:"elements"`
}

type Contact struct {
	ID     int    `json:"id"`
	Type   string `json:"type"`
	Value  string `json:"value"`
	Status string `json:"vrfStu"`
}

func esiaSubjectOID(subject json.RawMessage) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(subject))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", fmt.Errorf("разбор sub ID token: %w", err)
	}

	var oid string
	switch value := value.(type) {
	case string:
		oid = value
	case json.Number:
		oid = value.String()
	default:
		return "", fmt.Errorf("sub ID token имеет неподдерживаемый тип")
	}

	if oid == "" {
		return "", fmt.Errorf("sub ID token пуст")
	}
	for _, character := range oid {
		if character < '0' || character > '9' {
			return "", fmt.Errorf("sub ID token не является OID")
		}
	}

	return oid, nil
}

func (c Contacts) types() []string {
	types := make([]string, 0, len(c.Elements))
	seen := make(map[string]struct{})

	for _, contact := range c.Elements {
		if contact.Type == "" {
			continue
		}
		if _, exists := seen[contact.Type]; exists {
			continue
		}
		seen[contact.Type] = struct{}{}
		types = append(types, contact.Type)
	}

	return types
}

func (c Contacts) emailPresent() bool {
	return c.email() != ""
}

func (c Contacts) email() string {
	for _, contact := range c.Elements {
		if contact.Type == "EML" && strings.TrimSpace(contact.Value) != "" {
			return strings.TrimSpace(contact.Value)
		}
	}

	return ""
}

type esiaResourceClient struct {
	baseURI    string
	httpClient *http.Client
}

func newESIAResourceClient(baseURI string, httpClient *http.Client) *esiaResourceClient {
	return &esiaResourceClient{
		baseURI:    strings.TrimRight(baseURI, "/"),
		httpClient: httpClient,
	}
}

func (c *esiaResourceClient) getPerson(
	ctx context.Context,
	oid string,
	accessToken string,
) (Person, error) {
	var person Person
	if err := c.getJSON(
		ctx,
		"/rs/prns/"+url.PathEscape(oid),
		nil,
		accessToken,
		&person,
	); err != nil {
		return Person{}, fmt.Errorf("получение профиля ЕСИА: %w", err)
	}

	return person, nil
}

func (c *esiaResourceClient) getContacts(
	ctx context.Context,
	oid string,
	accessToken string,
) (Contacts, error) {
	var contacts Contacts
	if err := c.getJSON(
		ctx,
		"/rs/prns/"+url.PathEscape(oid)+"/ctts",
		url.Values{"embed": {"(elements)"}},
		accessToken,
		&contacts,
	); err != nil {
		return Contacts{}, fmt.Errorf("получение контактов ЕСИА: %w", err)
	}

	return contacts, nil
}

func (c *esiaResourceClient) getJSON(
	ctx context.Context,
	path string,
	query url.Values,
	accessToken string,
	target any,
) error {
	if c.httpClient == nil {
		return fmt.Errorf("не настроен HTTP client ЕСИА")
	}
	if c.baseURI == "" {
		return fmt.Errorf("не задан адрес ЕСИА")
	}
	if accessToken == "" {
		return fmt.Errorf("не получен access token ЕСИА")
	}

	endpoint := c.baseURI + path
	if len(query) != 0 {
		endpoint += "?" + query.Encode()
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("создание запроса ЕСИА: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+accessToken)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("выполнение запроса ЕСИА: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxESIAResourceErrorBody))
		return fmt.Errorf("ЕСИА вернула HTTP %d", response.StatusCode)
	}

	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return fmt.Errorf("разбор JSON-ответа ЕСИА: %w", err)
	}

	return nil
}
