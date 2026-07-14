package uploadhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"strings"
	"testing"
	"time"

	authapp "mathstudy/backend-go/internal/application/auth"
	uploadapp "mathstudy/backend-go/internal/application/upload"
	"mathstudy/backend-go/internal/domain/user"
)

func TestImageUploadRequiresAuthentication(t *testing.T) {
	handler := newTestHandler(t, &fakeUploadService{}, &fakeAuthenticator{})
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/upload")

	recorder := httptest.NewRecorder()
	request := multipartRequest(t, "/api/v1/upload/image", "image.png", "image/png", "data")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q", got)
	}
}

func TestImageUploadForwardsAuthenticatedFile(t *testing.T) {
	service := &fakeUploadService{imageResponse: uploadapp.Response{FileID: "file-1", URL: "/uploads/images/file-1.png", Filename: "file-1.png", ContentType: "image/png", Size: 4}}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/upload")

	recorder := httptest.NewRecorder()
	request := multipartRequest(t, "/api/v1/upload/image", "image.png", "image/png", "data")
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if service.imageContentType != "image/png" || service.imageSize != 4 || service.imageData != "data" {
		t.Fatalf("service call = contentType %q size %d data %q", service.imageContentType, service.imageSize, service.imageData)
	}
	var body uploadapp.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.FileID != "file-1" || body.URL != "/uploads/images/file-1.png" {
		t.Fatalf("body = %#v", body)
	}
}

func TestResourceUploadRequiresTeacherRole(t *testing.T) {
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, &fakeUploadService{}, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/upload")

	recorder := httptest.NewRecorder()
	request := multipartRequest(t, "/api/v1/upload/resource", "resource.pdf", "application/pdf", "pdf")
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestResourceUploadForwardsTeacherFile(t *testing.T) {
	service := &fakeUploadService{resourceResponse: uploadapp.Response{FileID: "file-2", URL: "/uploads/documents/file-2.pdf", Filename: "file-2.pdf", ContentType: "application/pdf", Size: 3}}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "teacher-1", Role: user.RoleTeacher}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/upload")

	recorder := httptest.NewRecorder()
	request := multipartRequest(t, "/api/v1/upload/resource", "resource.pdf", "application/pdf", "pdf")
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if service.resourceContentType != "application/pdf" || service.resourceSize != 3 || service.resourceData != "pdf" {
		t.Fatalf("service call = contentType %q size %d data %q", service.resourceContentType, service.resourceSize, service.resourceData)
	}
}

func TestResourceUploadMissingTokenReturnsBearerChallenge(t *testing.T) {
	handler := newTestHandler(t, &fakeUploadService{}, &fakeAuthenticator{})
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/upload")

	recorder := httptest.NewRecorder()
	request := multipartRequest(t, "/api/v1/upload/resource", "resource.pdf", "application/pdf", "pdf")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q", got)
	}
}

func TestUploadMapsServiceErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{name: "invalid type", err: uploadapp.ErrInvalidContentType, status: http.StatusUnsupportedMediaType, code: "INVALID_CONTENT_TYPE"},
		{name: "too large", err: uploadapp.ErrFileTooLarge, status: http.StatusRequestEntityTooLarge, code: "FILE_TOO_LARGE"},
		{name: "internal", err: errors.New("store failed"), status: http.StatusInternalServerError, code: "INTERNAL_ERROR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
			handler := newTestHandler(t, &fakeUploadService{imageErr: tt.err}, auth)
			mux := http.NewServeMux()
			handler.Register(mux, "/api/v1/upload")

			recorder := httptest.NewRecorder()
			request := multipartRequest(t, "/api/v1/upload/image", "image.png", "image/png", "data")
			request.Header.Set("Authorization", "Bearer token")
			mux.ServeHTTP(recorder, request)

			if recorder.Code != tt.status {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			var body map[string]string
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			if body["code"] != tt.code {
				t.Fatalf("body = %#v", body)
			}
		})
	}
}

func TestInternalErrorsRedactLogs(t *testing.T) {
	var logBuffer bytes.Buffer
	service := &fakeUploadService{imageErr: errors.New("store failed Authorization: Bearer upload-secret token=query-token api_key=plain password=letmein")}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler, err := NewHandler(slog.New(slog.NewTextHandler(&logBuffer, nil)), service, auth)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/upload")

	recorder := httptest.NewRecorder()
	request := multipartRequest(t, "/api/v1/upload/image", "image.png", "image/png", "data")
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	assertNoUploadCredentialLeak(t, recorder.Body.String())
	assertNoUploadCredentialLeak(t, logBuffer.String())
}

func TestUploadRejectsMissingMultipartFile(t *testing.T) {
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, &fakeUploadService{}, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/upload")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.Close(); err != nil {
		t.Fatalf("Close multipart writer: %v", err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/upload/image", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestUploadRateLimitUsesAuthenticatedUser(t *testing.T) {
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, &fakeUploadService{}, auth)
	handler.limiter = newUploadRateLimiter(1, time.Minute)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/upload")

	first := httptest.NewRecorder()
	firstRequest := multipartRequest(t, "/api/v1/upload/image", "image.png", "image/png", "data")
	firstRequest.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(first, firstRequest)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, body = %s", first.Code, first.Body.String())
	}

	second := httptest.NewRecorder()
	secondRequest := multipartRequest(t, "/api/v1/upload/image", "image.png", "image/png", "data")
	secondRequest.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(second, secondRequest)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, body = %s", second.Code, second.Body.String())
	}
}

func TestNewHandlerRejectsMissingDependencies(t *testing.T) {
	if _, err := NewHandler(nil, nil, &fakeAuthenticator{}); err == nil {
		t.Fatal("NewHandler(nil service) error = nil, want error")
	}
	if _, err := NewHandler(nil, &fakeUploadService{}, nil); err == nil {
		t.Fatal("NewHandler(nil auth) error = nil, want error")
	}
	if _, err := NewHandler(nil, &fakeUploadService{}, &fakeAuthenticator{}, WithRedisRateLimit(nil, 0)); err == nil {
		t.Fatal("NewHandler(invalid rate limit) error = nil, want error")
	}
}

func newTestHandler(t *testing.T, service Service, auth Authenticator) *Handler {
	t.Helper()
	handler, err := NewHandler(slog.New(slog.NewTextHandler(os.Stdout, nil)), service, auth)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	return handler
}

func assertNoUploadCredentialLeak(t *testing.T, value string) {
	t.Helper()
	for _, leaked := range []string{"upload-secret", "token=query-token", "api_key=plain", "password=letmein", "Bearer upload-secret"} {
		if strings.Contains(value, leaked) {
			t.Fatalf("value leaked %q in %q", leaked, value)
		}
	}
}

func multipartRequest(t *testing.T, target string, filename string, contentType string, content string) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatalf("CreatePart() error = %v", err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatalf("Write part error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close multipart writer: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, target, body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

type fakeAuthenticator struct {
	principal authapp.Principal
}

func (a *fakeAuthenticator) DecodeAccessToken(string) (authapp.Principal, bool) {
	if a.principal.UserID == "" {
		return authapp.Principal{}, false
	}
	return a.principal, true
}

type fakeUploadService struct {
	imageResponse       uploadapp.Response
	resourceResponse    uploadapp.Response
	imageErr            error
	resourceErr         error
	imageContentType    string
	imageSize           int64
	imageData           string
	resourceContentType string
	resourceSize        int64
	resourceData        string
}

func (s *fakeUploadService) SaveImage(_ context.Context, reader io.Reader, meta uploadapp.FileMeta) (uploadapp.Response, error) {
	data, _ := io.ReadAll(reader)
	s.imageData = string(data)
	s.imageContentType = meta.ContentType
	s.imageSize = meta.Size
	if s.imageErr != nil {
		return uploadapp.Response{}, s.imageErr
	}
	return s.imageResponse, nil
}

func (s *fakeUploadService) SaveResourceFile(_ context.Context, reader io.Reader, meta uploadapp.FileMeta) (uploadapp.Response, error) {
	data, _ := io.ReadAll(reader)
	s.resourceData = string(data)
	s.resourceContentType = meta.ContentType
	s.resourceSize = meta.Size
	if s.resourceErr != nil {
		return uploadapp.Response{}, s.resourceErr
	}
	return s.resourceResponse, nil
}
