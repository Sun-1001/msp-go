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
