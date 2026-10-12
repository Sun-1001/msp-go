package main

import (
	"log/slog"

	classroomhttp "mathstudy/backend/internal/adapter/http/classroom"
	teacherhttp "mathstudy/backend/internal/adapter/http/teacher"
	adapterpostgres "mathstudy/backend/internal/adapter/postgres"
	classroomapp "mathstudy/backend/internal/application/classroom"
	teacherapp "mathstudy/backend/internal/application/teacher"
)

type classroomModule struct {
	classRepo      adapterpostgres.ClassRepository
	classService   *classroomapp.Service
	classHandler   *classroomhttp.Handler
	teacherRepo    adapterpostgres.TeacherRepository
	teacherService *teacherapp.Service
	teacherHandler *teacherhttp.Handler
}

func wireClassroom(logger *slog.Logger, infra *infrastructure, auth *authModule) (*classroomModule, error) {
	classRepo, err := adapterpostgres.NewClassRepository(infra.db)
	if err != nil {
		return nil, stepFailed("configure class repository", err)
	}
	classService, err := classroomapp.NewService(classRepo)
	if err != nil {
		return nil, stepFailed("configure class service", err)
	}
	classHandler, err := classroomhttp.NewHandler(logger, classService, auth.service)
	if err != nil {
		return nil, stepFailed("configure class handler", err)
	}
	teacherRepo, err := adapterpostgres.NewTeacherRepository(infra.db)
	if err != nil {
		return nil, stepFailed("configure teacher repository", err)
	}
	teacherService, err := teacherapp.NewService(teacherRepo)
	if err != nil {
		return nil, stepFailed("configure teacher service", err)
	}
	teacherHandler, err := teacherhttp.NewHandler(logger, teacherService, auth.service)
	if err != nil {
		return nil, stepFailed("configure teacher handler", err)
	}
	return &classroomModule{
		classRepo:      classRepo,
		classService:   classService,
		classHandler:   classHandler,
		teacherRepo:    teacherRepo,
		teacherService: teacherService,
		teacherHandler: teacherHandler,
	}, nil
}
