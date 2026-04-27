package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	classroomapp "mathstudy/backend-go/internal/application/classroom"
	"mathstudy/backend-go/internal/domain/user"
)

func TestClassRepositoryIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("MSP_GO_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("set MSP_GO_TEST_DATABASE_URL to run PostgreSQL class repository integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New() error = %v", err)
	}
	defer pool.Close()

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatalf("BeginTx() error = %v", err)
	}
	defer tx.Rollback(context.Background())

	userRepo, err := NewUserRepository(tx)
	if err != nil {
		t.Fatalf("NewUserRepository() error = %v", err)
	}
	classRepo, err := NewClassRepository(tx)
	if err != nil {
		t.Fatalf("NewClassRepository() error = %v", err)
	}

	suffix := time.Now().UnixNano()
	code := fmt.Sprintf("C%011d", suffix%100000000000)
	displayName := "张老师"
	now := time.Now().UTC()
	teacher, err := userRepo.Create(ctx, user.CreateUser{
		ID:             fmt.Sprintf("teacher-%d", suffix),
		Username:       fmt.Sprintf("teacher_%d", suffix),
		Email:          fmt.Sprintf("teacher_%d@example.com", suffix),
		HashedPassword: "$2b$12$9x6kJZ77Z6u3Kz7Rkcl0Wuzx6E2UL6zLGCbyjEtW0QHfWkq0hPcN2",
		Role:           user.RoleTeacher,
		DisplayName:    &displayName,
		IsActive:       true,
		Status:         user.StatusActive,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	if err != nil {
		t.Fatalf("Create(teacher) error = %v", err)
	}
	student, err := userRepo.Create(ctx, user.CreateUser{
		ID:             fmt.Sprintf("student-%d", suffix),
		Username:       fmt.Sprintf("student_%d", suffix),
		Email:          fmt.Sprintf("student_%d@example.com", suffix),
		HashedPassword: "$2b$12$9x6kJZ77Z6u3Kz7Rkcl0Wuzx6E2UL6zLGCbyjEtW0QHfWkq0hPcN2",
		Role:           user.RoleStudent,
		IsActive:       true,
		Status:         user.StatusActive,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	if err != nil {
		t.Fatalf("Create(student) error = %v", err)
	}

	description := "竞赛班"
	classInfo, err := classRepo.CreateClass(ctx, classroomapp.ClassCreate{
		ID:          fmt.Sprintf("class-%d", suffix),
		Name:        "高一三班",
		Code:        code,
		TeacherID:   teacher.ID,
		Description: &description,
	}, now)
	if err != nil {
		t.Fatalf("CreateClass() error = %v", err)
	}
	if classInfo.Code != code || classInfo.Description == nil || *classInfo.Description != description {
		t.Fatalf("classInfo = %#v", classInfo)
	}
	if _, err := classRepo.CreateClass(ctx, classroomapp.ClassCreate{
		ID:        fmt.Sprintf("class-duplicate-%d", suffix),
		Name:      "重复班级号",
		Code:      code,
		TeacherID: teacher.ID,
	}, now); !errors.Is(err, classroomapp.ErrConflict) {
		t.Fatalf("CreateClass(duplicate) error = %v, want ErrConflict", err)
	}

	list, err := classRepo.ListTeacherClasses(ctx, teacher.ID)
	if err != nil {
		t.Fatalf("ListTeacherClasses() error = %v", err)
	}
	if len(list) != 1 || list[0].StudentCount == nil || *list[0].StudentCount != 0 {
		t.Fatalf("ListTeacherClasses() = %#v", list)
	}

	lookup, teacherRef, ok, err := classRepo.LookupClassByCode(ctx, strings.ToLower(code))
	if err != nil {
		t.Fatalf("LookupClassByCode() error = %v", err)
	}
	if !ok || lookup.ID != classInfo.ID || teacherRef == nil || teacherRef.DisplayName == nil || *teacherRef.DisplayName != displayName {
		t.Fatalf("LookupClassByCode() = %#v teacher=%#v ok=%t", lookup, teacherRef, ok)
	}

	if enrolled, err := classRepo.StudentHasEnrollment(ctx, student.ID); err != nil || enrolled {
		t.Fatalf("StudentHasEnrollment(before) = %t, %v", enrolled, err)
	}
	if err := classRepo.CreateEnrollment(ctx, classInfo.ID, student.ID, now); err != nil {
		t.Fatalf("CreateEnrollment() error = %v", err)
	}
	if enrolled, err := classRepo.StudentHasEnrollment(ctx, student.ID); err != nil || !enrolled {
		t.Fatalf("StudentHasEnrollment(after) = %t, %v", enrolled, err)
	}

	currentClass, ok, err := classRepo.GetStudentClass(ctx, student.ID)
	if err != nil {
		t.Fatalf("GetStudentClass() error = %v", err)
	}
	if !ok || currentClass.StudentCount == nil || *currentClass.StudentCount != 1 || currentClass.JoinedAt == nil {
		t.Fatalf("GetStudentClass() = %#v ok=%t", currentClass, ok)
	}

	detail, students, ok, err := classRepo.GetTeacherClassDetail(ctx, teacher.ID, classInfo.ID)
	if err != nil {
		t.Fatalf("GetTeacherClassDetail() error = %v", err)
	}
	if !ok || detail.ID != classInfo.ID || len(students) != 1 || students[0].ID != student.ID {
		t.Fatalf("GetTeacherClassDetail() = %#v students=%#v ok=%t", detail, students, ok)
	}

	removed, err := classRepo.RemoveStudent(ctx, teacher.ID, classInfo.ID, student.ID)
	if err != nil {
		t.Fatalf("RemoveStudent() error = %v", err)
	}
	if !removed {
		t.Fatal("RemoveStudent() = false, want true")
	}
	if left, err := classRepo.LeaveClass(ctx, student.ID); err != nil || left {
		t.Fatalf("LeaveClass(after remove) = %t, %v", left, err)
	}

	if err := classRepo.CreateEnrollment(ctx, classInfo.ID, student.ID, now); err != nil {
		t.Fatalf("CreateEnrollment(second) error = %v", err)
	}
	disbanded, err := classRepo.DisbandClass(ctx, teacher.ID, classInfo.ID)
	if err != nil {
		t.Fatalf("DisbandClass() error = %v", err)
	}
	if !disbanded {
		t.Fatal("DisbandClass() = false, want true")
	}
	if _, _, ok, err := classRepo.GetTeacherClassDetail(ctx, teacher.ID, classInfo.ID); err != nil || ok {
		t.Fatalf("GetTeacherClassDetail(after disband) ok=%t err=%v", ok, err)
	}
}
