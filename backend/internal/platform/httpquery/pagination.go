package httpquery

import (
	"errors"
	"net/url"
	"strconv"
)

const (
	PageField     = "page"
	PageSizeField = "page_size"
)

// PaginationParams contains parsed list pagination values.
type PaginationParams struct {
	Page     int
	PageSize int
}

// PaginationError reports which pagination field failed and why.
type PaginationError struct {
	Field string
	Err   error
}

func (e PaginationError) Error() string {
	if e.Field == "" {
		return e.Err.Error()
	}
	return e.Field + ": " + e.Err.Error()
}

func (e PaginationError) Unwrap() error {
	return e.Err
}

// Pagination parses page/page_size query parameters and enforces bounds.
func Pagination(query url.Values, defaultPageSize int, maxPageSize int) (PaginationParams, error) {
	page, err := Int(query.Get(PageField), 1)
	if err != nil {
		return PaginationParams{}, PaginationError{Field: PageField, Err: err}
	}
	if page < 1 {
		return PaginationParams{}, PaginationError{Field: PageField, Err: ErrIntOutOfRange}
	}
	pageSize, err := Int(query.Get(PageSizeField), defaultPageSize)
	if err != nil {
		return PaginationParams{}, PaginationError{Field: PageSizeField, Err: err}
	}
	if pageSize < 1 || pageSize > maxPageSize {
		return PaginationParams{}, PaginationError{Field: PageSizeField, Err: ErrIntOutOfRange}
	}
	return PaginationParams{Page: page, PageSize: pageSize}, nil
}

// PaginationErrorMessage returns the stable Chinese validation message used by HTTP adapters.
func PaginationErrorMessage(err error, maxPageSize int) string {
	var paginationErr PaginationError
	if !errors.As(err, &paginationErr) {
		return "分页参数无效"
	}
	if errors.Is(err, ErrInvalidInt) {
		return paginationErr.Field + " 必须是整数"
	}
	if paginationErr.Field == PageField {
		return "page 必须大于等于 1"
	}
	return "page_size 必须在 1 到 " + strconv.Itoa(maxPageSize) + " 之间"
}
