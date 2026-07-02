package httpjson

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

var ErrTrailingData = errors.New("json request body contains trailing data")

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
