package classroom

import (
	"context"
	"errors"
	"testing"
	"time"

	"mathstudy/backend-go/internal/domain/user"
)

func TestCreateClassRequiresTeacherOrAdmin(t *testing.T) {
	repo := &fakeClassRepo{user: UserRef{ID: "student-1", Role: user.RoleStudent}, userFound: true}
	service := newTestService(repo, time.Now(), "ABC123")

	_, err := service.CreateClass(context.Background(), "student-1", "高一三班", nil)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("CreateClass() error = %v, want ErrForbidden", err)
	}
}

func TestCreateClassRetriesCodeConflict(t *testing.T) {
	now := time.Date(2026, time.April, 27, 10, 0, 0, 0, time.UTC)
	repo := &fakeClassRepo{
		user:            UserRef{ID: "teacher-1", Role: user.RoleTeacher},
		userFound:       true,
		createConflicts: 1,
		createResponse:  ClassInfo{ID: "class-1", Name: "高一三班", Code: "ZZ9999"},
	}
	service := newTestService(repo, now, "AA1111", "ZZ9999")

	response, err := service.CreateClass(context.Background(), "teacher-1", " 高一三班 ", nil)
	if err != nil {
		t.Fatalf("CreateClass() error = %v", err)
	}
	if !response.Success || response.Message != "班级创建成功" || response.ClassInfo.ID != "class-1" {
		t.Fatalf("response = %#v", response)
	}
	if repo.createCalls != 2 || repo.lastCreate.Code != "ZZ9999" || repo.lastCreate.Name != "高一三班" || !repo.lastNow.Equal(now) {
		t.Fatalf("create calls=%d input=%#v now=%v", repo.createCalls, repo.lastCreate, repo.lastNow)
	}
}

func TestJoinClassChecksStudentAndExistingEnrollment(t *testing.T) {
	repo := &fakeClassRepo{user: UserRef{ID: "teacher-1", Role: user.RoleTeacher}, userFound: true}
	service := newTestService(repo, time.Now(), "ABC123")
	if _, err := service.JoinClass(context.Background(), "teacher-1", "ABC123"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("JoinClass(non-student) error = %v", err)
	}

	repo = &fakeClassRepo{user: UserRef{ID: "student-1", Role: user.RoleStudent}, userFound: true, hasEnrollment: true}
	service = newTestService(repo, time.Now(), "ABC123")
	if _, err := service.JoinClass(context.Background(), "student-1", "ABC123"); !errors.Is(err, ErrConflict) {
		t.Fatalf("JoinClass(enrolled) error = %v", err)
	}
}

func TestJoinClassNormalizesCodeAndCreatesEnrollment(t *testing.T) {
	now := time.Date(2026, time.April, 27, 11, 0, 0, 0, time.UTC)
	repo := &fakeClassRepo{
		user:        UserRef{ID: "student-1", Role: user.RoleStudent},
		userFound:   true,
		lookupClass: ClassInfo{ID: "class-1", Code: "ABC123"},
		lookupFound: true,
	}
	service := newTestService(repo, now, "ABC123")

	response, err := service.JoinClass(context.Background(), "student-1", " abc123 ")
	if err != nil {
		t.Fatalf("JoinClass() error = %v", err)
	}
	if !response.Success || response.ClassInfo.ID != "class-1" || repo.lastLookupCode != "ABC123" {
		t.Fatalf("response=%#v lookup=%q", response, repo.lastLookupCode)
	}
	if repo.lastEnrollmentClassID != "class-1" || repo.lastEnrollmentStudentID != "student-1" || !repo.lastNow.Equal(now) {
		t.Fatalf("enrollment class=%q student=%q now=%v", repo.lastEnrollmentClassID, repo.lastEnrollmentStudentID, repo.lastNow)
	}
}

func TestNotFoundActions(t *testing.T) {
	service := newTestService(&fakeClassRepo{}, time.Now(), "ABC123")
	if _, err := service.GetTeacherClassDetail(context.Background(), "teacher-1", "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetTeacherClassDetail() error = %v", err)
	}
	if _, err := service.RemoveStudent(context.Background(), "teacher-1", "class-1", "student-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("RemoveStudent() error = %v", err)
	}
	if _, err := service.DisbandClass(context.Background(), "teacher-1", "class-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DisbandClass() error = %v", err)
	}
	if _, err := service.LeaveClass(context.Background(), "student-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("LeaveClass() error = %v", err)
	}
}

func TestGetStudentClassReturnsNullWhenMissing(t *testing.T) {
	service := newTestService(&fakeClassRepo{}, time.Now(), "ABC123")
	response, err := service.GetStudentClass(context.Background(), "student-1")
	if err != nil {
		t.Fatalf("GetStudentClass() error = %v", err)
	}
	if response.ClassInfo != nil {
		t.Fatalf("ClassInfo = %#v, want nil", response.ClassInfo)
	}
}

func TestNewServiceRejectsNilRepository(t *testing.T) {
	if _, err := NewService(nil); err == nil {
		t.Fatal("NewService(nil) error = nil, want error")
	}
}

func newTestService(repo Repository, now time.Time, codes ...string) *Service {
	service, err := NewService(repo)
	if err != nil {
		panic(err)
	}
	service.now = func() time.Time { return now }
	index := 0
	service.codeFactory = func() (string, error) {
		if index >= len(codes) {
			return "ABC123", nil
		}
		code := codes[index]
		index++
		return code, nil
	}
	return service
}

type fakeClassRepo struct {
	user                    UserRef
	userFound               bool
	createResponse          ClassInfo
	createConflicts         int
	classes                 []ClassInfo
	detailClass             ClassInfo
	detailStudents          []StudentItem
	detailFound             bool
	removeOK                bool
	disbandOK               bool
	lookupClass             ClassInfo
	lookupTeacher           *UserRef
	lookupFound             bool
	hasEnrollment           bool
	leaveOK                 bool
	studentClass            ClassInfo
	studentClassFound       bool
	createCalls             int
	lastCreate              ClassCreate
	lastNow                 time.Time
	lastLookupCode          string
	lastEnrollmentClassID   string
	lastEnrollmentStudentID string
}

func (r *fakeClassRepo) GetUser(context.Context, string) (UserRef, bool, error) {
	return r.user, r.userFound, nil
}

func (r *fakeClassRepo) CreateClass(_ context.Context, input ClassCreate, now time.Time) (ClassInfo, error) {
	r.createCalls++
	r.lastCreate = input
	r.lastNow = now
	if r.createConflicts > 0 {
		r.createConflicts--
		return ClassInfo{}, ErrConflict
	}
	return r.createResponse, nil
}

func (r *fakeClassRepo) ListTeacherClasses(context.Context, string) ([]ClassInfo, error) {
	return r.classes, nil
}

func (r *fakeClassRepo) GetTeacherClassDetail(context.Context, string, string) (ClassInfo, []StudentItem, bool, error) {
	return r.detailClass, r.detailStudents, r.detailFound, nil
}

func (r *fakeClassRepo) RemoveStudent(context.Context, string, string, string) (bool, error) {
	return r.removeOK, nil
}

func (r *fakeClassRepo) DisbandClass(context.Context, string, string) (bool, error) {
	return r.disbandOK, nil
}

func (r *fakeClassRepo) LookupClassByCode(_ context.Context, code string) (ClassInfo, *UserRef, bool, error) {
	r.lastLookupCode = code
	return r.lookupClass, r.lookupTeacher, r.lookupFound, nil
}

func (r *fakeClassRepo) StudentHasEnrollment(context.Context, string) (bool, error) {
	return r.hasEnrollment, nil
}

func (r *fakeClassRepo) CreateEnrollment(_ context.Context, classID string, studentID string, now time.Time) error {
	r.lastEnrollmentClassID = classID
	r.lastEnrollmentStudentID = studentID
	r.lastNow = now
	return nil
}

func (r *fakeClassRepo) LeaveClass(context.Context, string) (bool, error) {
	return r.leaveOK, nil
}

func (r *fakeClassRepo) GetStudentClass(context.Context, string) (ClassInfo, bool, error) {
	return r.studentClass, r.studentClassFound, nil
}
