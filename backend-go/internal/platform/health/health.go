package health

import (
	"context"
	"time"
)

// Pinger is implemented by dependencies that can verify connectivity.
type Pinger interface {
	Ping(context.Context) error
}

// RedisPingerFunc adapts a function into a Pinger.
type RedisPingerFunc func(context.Context) error

// Ping calls f(ctx).
func (f RedisPingerFunc) Ping(ctx context.Context) error {
	return f(ctx)
}

// Checker evaluates process and dependency health.
type Checker struct {
	version string
	db      Pinger
	redis   Pinger
}

// ComponentStatus describes one health component.
type ComponentStatus struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// DetailedStatus is returned by /health/detailed.
type DetailedStatus struct {
	Status     string                     `json:"status"`
	Version    string                     `json:"version"`
	CheckedAt  time.Time                  `json:"checked_at"`
	Components map[string]ComponentStatus `json:"components"`
}

// NewChecker creates a dependency health checker.
func NewChecker(version string, db Pinger, redis Pinger) Checker {
	return Checker{version: version, db: db, redis: redis}
}

// Simple returns the lightweight health payload.
func (c Checker) Simple() map[string]string {
	return map[string]string{
		"status":  "healthy",
		"version": c.version,
	}
}

// Detailed checks PostgreSQL and Redis with a short timeout.
func (c Checker) Detailed(ctx context.Context) DetailedStatus {
	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	components := map[string]ComponentStatus{
		"app": {Status: "healthy"},
	}
	status := "healthy"

	if c.db != nil {
		components["postgres"] = pingComponent(checkCtx, c.db)
		if components["postgres"].Status != "healthy" {
			status = "degraded"
		}
	}
	if c.redis != nil {
		components["redis"] = pingComponent(checkCtx, c.redis)
		if components["redis"].Status != "healthy" {
			status = "degraded"
		}
	}

	return DetailedStatus{
		Status:     status,
		Version:    c.version,
		CheckedAt:  time.Now().UTC(),
		Components: components,
	}
}

func pingComponent(ctx context.Context, pinger Pinger) ComponentStatus {
	if err := pinger.Ping(ctx); err != nil {
		return ComponentStatus{Status: "unhealthy", Error: err.Error()}
	}
	return ComponentStatus{Status: "healthy"}
}
