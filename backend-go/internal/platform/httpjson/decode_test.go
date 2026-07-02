package httpjson

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeStrictAcceptsSingleJSONDocument(t *testing.T) {
	var payload struct {
		Name string `json:"name"`
	}
	request := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"alice"}`))

	if err := DecodeStrict(httptest.NewRecorder(), request, 1<<20, &payload); err != nil {
		t.Fatalf("DecodeStrict() error = %v", err)
	}
	if payload.Name != "alice" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestDecodeStrictRejectsTrailingJSONDocument(t *testing.T) {
	var payload struct {
		Name string `json:"name"`
	}
	request := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"alice"} {"name":"bob"}`))

	err := DecodeStrict(httptest.NewRecorder(), request, 1<<20, &payload)
	if !errors.Is(err, ErrTrailingData) {
		t.Fatalf("DecodeStrict() error = %v, want ErrTrailingData", err)
	}
}

func TestDecodeStrictRejectsTrailingGarbage(t *testing.T) {
	var payload struct {
		Name string `json:"name"`
	}
	request := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"alice"} garbage`))

	if err := DecodeStrict(httptest.NewRecorder(), request, 1<<20, &payload); err == nil {
		t.Fatal("DecodeStrict() error = nil, want error")
	}
}

func TestDecodeStrictRejectsOversizedBody(t *testing.T) {
	var payload struct {
		Name string `json:"name"`
	}
	request := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"alice"}`))

	if err := DecodeStrict(httptest.NewRecorder(), request, 4, &payload); err == nil {
		t.Fatal("DecodeStrict() error = nil, want error")
	}
}
