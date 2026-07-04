package httpjson

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

var (
	ErrTrailingData = errors.New("json request body contains trailing data")
	ErrBodyTooLarge = errors.New("json body exceeds size limit")
)

// DetailError is the common error response shape used by HTTP adapters.
type DetailError struct {
	Detail  string `json:"detail"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func DecodeStrict(w http.ResponseWriter, r *http.Request, maxBytes int64, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBytes))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return ErrTrailingData
		}
		return err
	}
	return nil
}

// DecodeStrictOrDetailError decodes a strict JSON request body and writes the common 422 detail error on failure.
func DecodeStrictOrDetailError(w http.ResponseWriter, r *http.Request, maxBytes int64, target any) bool {
	if err := DecodeStrict(w, r, maxBytes, target); err != nil {
		WriteDetailError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "请求体格式错误")
		return false
	}
	return true
}

// DecodeStrictOrBadRequest decodes a strict JSON request body and writes the common 400 bad JSON detail error on failure.
func DecodeStrictOrBadRequest(w http.ResponseWriter, r *http.Request, maxBytes int64, target any) bool {
	if err := DecodeStrict(w, r, maxBytes, target); err != nil {
		WriteDetailError(w, http.StatusBadRequest, "BAD_REQUEST", "请求体不是有效 JSON")
		return false
	}
	return true
}

func DecodeLimited(reader io.Reader, maxBytes int64, target any) error {
	decoder := json.NewDecoder(&limitedReader{reader: reader, remaining: maxBytes})
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return ErrTrailingData
		}
		return err
	}
	return nil
}

func Write(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func WriteDetailError(w http.ResponseWriter, status int, code string, message string) {
	Write(w, status, DetailError{Detail: message, Code: code, Message: message})
}

type limitedReader struct {
	reader    io.Reader
	remaining int64
}

func (r *limitedReader) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		var probe [1]byte
		n, err := r.reader.Read(probe[:])
		if n > 0 {
			return 0, ErrBodyTooLarge
		}
		if err != nil {
			return 0, err
		}
		return 0, io.EOF
	}
	if int64(len(p)) > r.remaining {
		p = p[:int(r.remaining)]
	}
	n, err := r.reader.Read(p)
	r.remaining -= int64(n)
	return n, err
}
