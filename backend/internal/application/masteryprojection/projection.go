// Package masteryprojection owns the read-side DKT forgetting projection shared by learning views.
package masteryprojection

import (
	"math"
	"time"

	"mathstudy/backend/internal/platform/numutil"
)

const (
	retentionFloor = 0.05
	decayRate      = 0.05
)

// Current projects a stored DKT probability to now without mutating persisted state.
func Current(mastery float64, lastAttemptAt *time.Time, now time.Time) float64 {
	if lastAttemptAt == nil {
		return mastery
	}
	daysSinceLast := now.Sub(*lastAttemptAt).Hours() / 24
	if daysSinceLast <= 0 || mastery <= retentionFloor {
		return mastery
	}
	decayed := retentionFloor + (mastery-retentionFloor)*math.Exp(-decayRate*daysSinceLast)
	return numutil.ClampFloat(decayed, 0.001, 0.999)
}
