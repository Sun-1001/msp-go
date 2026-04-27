package classroomhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	authapp "mathstudy/backend-go/internal/application/auth"
	classroomapp "mathstudy/backend-go/internal/application/classroom"
	"mathstudy/backend-go/internal/domain/user"
)

func TestListRequiresBearerToken(t *testing.T) {
	handler := newTestHandler(t, &fakeClassService{}, &fakeAuthenticator{})
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/classes")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/classes/teacher", nil)
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q", got)
	}
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["detail"] != "未认证，请先登录" || body["code"] != "UNAUTHORIZED" {
		t.Fatalf("body = %#v", body)
	}
}

func TestCreateRequiresTeacherRole(t *testing.T) {
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, &fakeClassService{}, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/classes")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/classes", bytes.NewBufferString(`{"name":"高一三班"}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestCreateValidatesAndForwardsRequest(t *testing.T) {
	description := "竞赛班"
	service := &fakeClassService{createResponse: classroomapp.ClassCreateResponse{Success: true, Message: "班级创建成功", ClassInfo: classroomapp.ClassInfo{ID: "class-1"}}}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "teacher-1", Role: user.RoleTeacher}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/classes")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/classes", bytes.NewBufferString(`{"name":"高一三班","description":"竞赛班"}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if service.lastTeacherID != "teacher-1" || service.lastName != "高一三班" || service.lastDescription == nil || *service.lastDescription != description {
		t.Fatalf("teacher=%q name=%q description=%v", service.lastTeacherID, service.lastName, service.lastDescription)
	}
}

func TestTeacherRoutesForwardPathValues(t *testing.T) {
	service := &fakeClassService{
		detailResponse:  classroomapp.ClassDetailResponse{ClassInfo: classroomapp.ClassInfo{ID: "class-1"}},
		removeResponse:  classroomapp.ActionResponse{Success: true, Message: "学生已移除"},
		disbandResponse: classroomapp.ActionResponse{Success: true, Message: "班级已解散"},
	}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "teacher-1", Role: user.RoleTeacher}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/classes")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/classes/teacher/class-1", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.lastClassID != "class-1" {
		t.Fatalf("detail status=%d class=%q", recorder.Code, service.lastClassID)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodDelete, "/api/v1/classes/teacher/class-1/students/student-1", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.lastStudentID != "student-1" {
		t.Fatalf("remove status=%d student=%q", recorder.Code, service.lastStudentID)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodDelete, "/api/v1/classes/teacher/class-2", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || service.lastClassID != "class-2" {
		t.Fatalf("disband status=%d class=%q", recorder.Code, service.lastClassID)
	}
}

func TestStudentRoutesRequireStudentRole(t *testing.T) {
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "teacher-1", Role: user.RoleTeacher}}
	handler := newTestHandler(t, &fakeClassService{}, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/classes")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/classes/me", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestLookupAndJoinValidateCode(t *testing.T) {
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, &fakeClassService{}, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/classes")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/classes/lookup?code=abc", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("lookup status = %d", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/v1/classes/join", bytes.NewBufferString(`{"code":"ABC123"}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("join status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if service := handler.service.(*fakeClassService); service.lastStudentID != "student-1" || service.lastCode != "ABC123" {
		t.Fatalf("student=%q code=%q", service.lastStudentID, service.lastCode)
	}
}

func TestNotFoundErrors(t *testing.T) {
	service := &fakeClassService{
		detailErr:  classroomapp.ErrNotFound,
		removeErr:  classroomapp.ErrNotFound,
		disbandErr: classroomapp.ErrNotFound,
		joinErr:    classroomapp.ErrNotFound,
		leaveErr:   classroomapp.ErrNotFound,
	}
	auth := &fakeAuthenticator{principal: authapp.Principal{UserID: "student-1", Role: user.RoleStudent}}
	handler := newTestHandler(t, service, auth)
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/classes")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/classes/join", bytes.NewBufferString(`{"code":"ABC123"}`))
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("join status = %d", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/v1/classes/leave", nil)
	request.Header.Set("Authorization", "Bearer token")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("leave status = %d", recorder.Code)
	}
}

func TestNewHandlerRejectsMissingDependencies(t *testing.T) {
	if _, err := NewHandler(nil, nil, &fakeAuthenticator{}); err == nil {
		t.Fatal("NewHandler(nil service) error = nil, want error")
	}
	if _, err := NewHandler(nil, &fakeClassService{}, nil); err == nil {
		t.Fatal("NewHandler(nil auth) error = nil, want error")
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

type fakeAuthenticator struct {
	principal authapp.Principal
}

func (a *fakeAuthenticator) DecodeAccessToken(string) (authapp.Principal, bool) {
	if a.principal.UserID == "" {
		return authapp.Principal{}, false
	}
	return a.principal, true
}

type fakeClassService struct {
	createResponse  classroomapp.ClassCreateResponse
	listResponse    classroomapp.ClassListResponse
	detailResponse  classroomapp.ClassDetailResponse
	removeResponse  classroomapp.ActionResponse
	disbandResponse classroomapp.ActionResponse
	lookupResponse  classroomapp.ClassLookupResponse
	joinResponse    classroomapp.JoinClassResponse
	leaveResponse   classroomapp.ActionResponse
	myClassResponse classroomapp.StudentClassResponse
	createErr       error
	listErr         error
	detailErr       error
	removeErr       error
	disbandErr      error
	lookupErr       error
	joinErr         error
	leaveErr        error
	myClassErr      error
	lastTeacherID   string
	lastStudentID   string
	lastClassID     string
	lastName        string
	lastDescription *string
	lastCode        string
}

func (s *fakeClassService) CreateClass(_ context.Context, teacherID string, name string, description *string) (classroomapp.ClassCreateResponse, error) {
	s.lastTeacherID = teacherID
	s.lastName = name
	s.lastDescription = description
	return s.createResponse, s.createErr
}

func (s *fakeClassService) ListTeacherClasses(_ context.Context, teacherID string) (classroomapp.ClassListResponse, error) {
	s.lastTeacherID = teacherID
	return s.listResponse, s.listErr
}

func (s *fakeClassService) GetTeacherClassDetail(_ context.Context, teacherID string, classID string) (classroomapp.ClassDetailResponse, error) {
	s.lastTeacherID = teacherID
	s.lastClassID = classID
	return s.detailResponse, s.detailErr
}

func (s *fakeClassService) RemoveStudent(_ context.Context, teacherID string, classID string, studentID string) (classroomapp.ActionResponse, error) {
	s.lastTeacherID = teacherID
	s.lastClassID = classID
	s.lastStudentID = studentID
	return s.removeResponse, s.removeErr
}

func (s *fakeClassService) DisbandClass(_ context.Context, teacherID string, classID string) (classroomapp.ActionResponse, error) {
	s.lastTeacherID = teacherID
	s.lastClassID = classID
	return s.disbandResponse, s.disbandErr
}

func (s *fakeClassService) LookupClass(_ context.Context, code string) (classroomapp.ClassLookupResponse, error) {
	s.lastCode = code
	return s.lookupResponse, s.lookupErr
}

func (s *fakeClassService) JoinClass(_ context.Context, studentID string, code string) (classroomapp.JoinClassResponse, error) {
	s.lastStudentID = studentID
	s.lastCode = code
	return s.joinResponse, s.joinErr
}

func (s *fakeClassService) LeaveClass(_ context.Context, studentID string) (classroomapp.ActionResponse, error) {
	s.lastStudentID = studentID
	return s.leaveResponse, s.leaveErr
}

func (s *fakeClassService) GetStudentClass(_ context.Context, studentID string) (classroomapp.StudentClassResponse, error) {
	s.lastStudentID = studentID
	return s.myClassResponse, s.myClassErr
}
