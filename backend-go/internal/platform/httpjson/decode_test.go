package httpjson

import (
	"errors"
	"net/http"
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

func TestDecodeStrictOrDetailError(t *testing.T) {
	var payload struct {
		Name string `json:"name"`
	}
	request := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"alice"}`))
	recorder := httptest.NewRecorder()

	if ok := DecodeStrictOrDetailError(recorder, request, 1<<20, &payload); !ok {
		t.Fatal("DecodeStrictOrDetailError() = false, want true")
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want untouched default 200", recorder.Code)
	}
	if payload.Name != "alice" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestDecodeStrictOrDetailErrorWritesCommonError(t *testing.T) {
	var payload struct {
		Name string `json:"name"`
	}
	request := httptest.NewRequest("POST", "/", strings.NewReader(`{`))
	recorder := httptest.NewRecorder()

	if ok := DecodeStrictOrDetailError(recorder, request, 1<<20, &payload); ok {
		t.Fatal("DecodeStrictOrDetailError() = true, want false")
	}
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", recorder.Code)
	}
	if got := strings.TrimSpace(recorder.Body.String()); got != `{"detail":"请求体格式错误","code":"VALIDATION_ERROR","message":"请求体格式错误"}` {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestDecodeStrictOrBadRequest(t *testing.T) {
	var payload struct {
		Name string `json:"name"`
	}
	request := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"alice"}`))
	recorder := httptest.NewRecorder()

	if ok := DecodeStrictOrBadRequest(recorder, request, 1<<20, &payload); !ok {
		t.Fatal("DecodeStrictOrBadRequest() = false, want true")
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want untouched default 200", recorder.Code)
	}
	if payload.Name != "alice" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestDecodeStrictOrBadRequestWritesCommonError(t *testing.T) {
	var payload struct {
		Name string `json:"name"`
	}
	request := httptest.NewRequest("POST", "/", strings.NewReader(`{`))
	recorder := httptest.NewRecorder()

	if ok := DecodeStrictOrBadRequest(recorder, request, 1<<20, &payload); ok {
		t.Fatal("DecodeStrictOrBadRequest() = true, want false")
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
	if got := strings.TrimSpace(recorder.Body.String()); got != `{"detail":"请求体不是有效 JSON","code":"BAD_REQUEST","message":"请求体不是有效 JSON"}` {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestDecodeLimitedAcceptsSingleJSONDocument(t *testing.T) {
	var payload struct {
		Name string `json:"name"`
	}

	if err := DecodeLimited(strings.NewReader(`{"name":"alice","extra":true}`), 1<<20, &payload); err != nil {
		t.Fatalf("DecodeLimited() error = %v", err)
	}
	if payload.Name != "alice" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestDecodeLimitedRejectsTrailingJSONDocument(t *testing.T) {
	var payload struct {
		Name string `json:"name"`
	}

	err := DecodeLimited(strings.NewReader(`{"name":"alice"} {"name":"bob"}`), 1<<20, &payload)
	if !errors.Is(err, ErrTrailingData) {
		t.Fatalf("DecodeLimited() error = %v, want ErrTrailingData", err)
	}
}

func TestDecodeLimitedRejectsOversizedDocument(t *testing.T) {
	var payload struct {
		Name string `json:"name"`
	}

	if err := DecodeLimited(strings.NewReader(`{"name":"alice"}`), 4, &payload); !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("DecodeLimited() error = %v, want ErrBodyTooLarge", err)
	}
}

func TestDecodeLimitedRejectsOversizedTrailingWhitespace(t *testing.T) {
	var payload struct {
		Name string `json:"name"`
	}
	document := `{"name":"alice"}`

	err := DecodeLimited(strings.NewReader(document+strings.Repeat(" ", 10)), int64(len(document)), &payload)
	if !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("DecodeLimited() error = %v, want ErrBodyTooLarge", err)
	}
}

func TestWriteSetsJSONResponse(t *testing.T) {
	recorder := httptest.NewRecorder()

	Write(recorder, 201, struct {
		Name string `json:"name"`
	}{Name: "alice"})

	if recorder.Code != 201 {
		t.Fatalf("status = %d, want 201", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := strings.TrimSpace(recorder.Body.String()); got != `{"name":"alice"}` {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestWriteDetailErrorSetsCommonShape(t *testing.T) {
	recorder := httptest.NewRecorder()

	WriteDetailError(recorder, 422, "VALIDATION_ERROR", "请求体格式错误")

	if recorder.Code != 422 {
		t.Fatalf("status = %d, want 422", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := strings.TrimSpace(recorder.Body.String()); got != `{"detail":"请求体格式错误","code":"VALIDATION_ERROR","message":"请求体格式错误"}` {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}
