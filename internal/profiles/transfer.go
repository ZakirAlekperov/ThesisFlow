package profiles

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
)

const MaxJSONBytes = 64 << 10

type transfer struct {
	Schema      int    `json:"schema"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Items       []Item `json:"items"`
}

// EncodeJSON deliberately contains no student IDs, DB IDs, local paths,
// or currently accepted files, only portable profile keys and titles.
// RU: JSON не переносит чужие ID, файловые пути и персональные сведения.
func EncodeJSON(p Profile) ([]byte, error) {
	return json.MarshalIndent(transfer{Schema: 1, Name: p.Name, Description: p.Description, Items: p.Items}, "", "  ")
}

// DecodeJSON refuses unknown fields, stale schema, trailing objects and
// oversized payloads before any database mutation.
// RU: Строго проверяем формат и дубликаты до импорта.
func DecodeJSON(data []byte) (Profile, error) {
	if len(data) == 0 || len(data) > MaxJSONBytes {
		return Profile{}, ErrInvalid
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var v transfer
	if err := dec.Decode(&v); err != nil {
		return Profile{}, ErrInvalid
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return Profile{}, ErrInvalid
	}
	if v.Schema != 1 {
		return Profile{}, ErrInvalid
	}
	p := Profile{Name: v.Name, Description: v.Description, Items: v.Items}
	if err := validateProfile(p); err != nil {
		return Profile{}, err
	}
	return p, nil
}

// ImportJSON creates a NEW profile only. Existing ones are never overwritten.
// RU: Импорт не обновляет существующий профиль без согласия.
func (s *Service) ImportJSON(ctx context.Context, data []byte) (Profile, error) {
	p, err := DecodeJSON(data)
	if err != nil {
		return Profile{}, err
	}
	return s.Create(ctx, p)
}
