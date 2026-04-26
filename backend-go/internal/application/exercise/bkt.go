package exercise

import "math"

const epsilon = 1e-6

type bktResult struct {
	mastery    float64
	confidence float64
}

func defaultBKTParams() BKTParams {
	return BKTParams{PL0: 0.25, PT: 0.12, PG: 0.20, PS: 0.10}
}

func normalizeBKT(params BKTParams) BKTParams {
	if params == (BKTParams{}) {
		params = defaultBKTParams()
	}
	return BKTParams{
		PL0: clamp(params.PL0, 0.001, 0.999),
		PT:  clamp(params.PT, 0.001, 0.6),
		PG:  clamp(params.PG, 0.001, 0.4),
		PS:  clamp(params.PS, 0.001, 0.4),
	}
}

func personalizeBKT(params BKTParams, preferredDifficulty float64, learningPace float64, itemDifficulty float64, errorType string) BKTParams {
	base := normalizeBKT(params)
	preferred := clamp(preferredDifficulty, 0, 1)
	difficulty := clamp(itemDifficulty, 0, 1)
	pace := math.Max(0.2, learningPace)

	difficultyBias := difficulty - preferred
	paceDelta := clamp(pace-1.0, -0.5, 0.8)

	personalized := BKTParams{
		PL0: base.PL0 + 0.10*(preferred-difficulty) + 0.05*paceDelta,
		PT:  base.PT * (1.0 + 0.35*paceDelta) * (1.0 - 0.15*math.Max(difficultyBias, 0)),
		PG:  base.PG + 0.08*difficultyBias,
		PS:  base.PS + 0.12*difficultyBias - 0.03*paceDelta,
	}
	switch errorType {
	case "calculation", "symbolic":
		personalized.PS += 0.04
	case "conceptual", "logical":
		personalized.PG -= 0.03
	}
	return normalizeBKT(personalized)
}

func bktUpdate(priorMastery float64, isCorrect bool, params BKTParams, attemptCount int) bktResult {
	normalized := normalizeBKT(params)
	prior := clamp(priorMastery, 0.001, 0.999)

	var numerator float64
	var denominator float64
	if isCorrect {
		numerator = prior * (1.0 - normalized.PS)
		denominator = numerator + (1.0-prior)*normalized.PG
	} else {
		numerator = prior * normalized.PS
		denominator = numerator + (1.0-prior)*(1.0-normalized.PG)
	}
	posteriorObserved := clamp(numerator/math.Max(denominator, epsilon), 0.001, 0.999)
	posteriorNext := clamp(posteriorObserved+(1.0-posteriorObserved)*normalized.PT, 0.001, 0.999)
	effectiveAttempts := math.Max(float64(attemptCount), 0) + 1
	confidence := clamp(1.0-math.Exp(-effectiveAttempts/6.0), 0.0, 1.0)
	return bktResult{mastery: posteriorNext, confidence: confidence}
}

func applyForgetting(mastery float64, daysSinceLast float64, floor float64) float64 {
	if floor == 0 {
		floor = 0.25
	}
	if daysSinceLast <= 0 || mastery <= floor {
		return mastery
	}
	decayed := floor + (mastery-floor)*math.Exp(-0.05*daysSinceLast)
	return clamp(decayed, 0.001, 0.999)
}
