package exercise

import (
	"hash/fnv"
	"math"
	"sort"
	"strings"

	"mathstudy/backend-go/internal/platform/numutil"
)

const (
	dktModelName         = "dkt-sakt-lite"
	dktMaxSequenceLength = 64
	dktEmbeddingDim      = 16
	dktRetentionFloor    = 0.05
	epsilon              = 1e-6
)

type dktResult struct {
	mastery         float64
	confidence      float64
	attentionWeight float64
	sequenceLength  int
}

func buildDKTSequence(history []LearningInteraction, current LearningInteraction) []LearningInteraction {
	sequence := make([]LearningInteraction, 0, len(history)+1)
	seenCurrent := false
	for _, item := range history {
		if strings.TrimSpace(item.ExerciseID) == "" {
			continue
		}
		item.ConceptIDs = uniqueNonEmpty(item.ConceptIDs)
		if len(item.ConceptIDs) == 0 {
			continue
		}
		item.Difficulty = numutil.ClampFloat(item.Difficulty, 0, 1)
		if item.ExerciseID == current.ExerciseID && item.SubmittedAt.Equal(current.SubmittedAt) {
			seenCurrent = true
		}
		sequence = append(sequence, item)
	}
	if !seenCurrent {
		sequence = append(sequence, current)
	}
	sort.SliceStable(sequence, func(i, j int) bool {
		return sequence[i].SubmittedAt.Before(sequence[j].SubmittedAt)
	})
	if len(sequence) > dktMaxSequenceLength {
		sequence = sequence[len(sequence)-dktMaxSequenceLength:]
	}
	return sequence
}

func dktColdStartMastery(preferredDifficulty float64, learningPace float64, itemDifficulty float64) float64 {
	preferred := numutil.ClampFloat(preferredDifficulty, 0, 1)
	difficulty := numutil.ClampFloat(itemDifficulty, 0, 1)
	pace := numutil.ClampFloat(learningPace, 0.2, 2.0)
	return numutil.ClampFloat(0.45+0.12*(preferred-difficulty)+0.05*(pace-1.0), 0.15, 0.75)
}

func dktUpdate(priorMastery float64, targetConcept string, current LearningInteraction, sequence []LearningInteraction, profile StudentProfile, errorType string, attemptCount int) dktResult {
	prior := numutil.ClampFloat(priorMastery, 0.001, 0.999)
	difficulty := numutil.ClampFloat(current.Difficulty, 0, 1)
	abilityGap := numutil.ClampFloat(profile.PreferredDifficulty-difficulty, -1, 1)
	pace := numutil.ClampFloat(profile.LearningPace, 0.2, 2.0)

	attentionSignal, attentionWeight := attentionSignal(targetConcept, current, sequence)
	currentGain := 0.18 + 0.05*abilityGap + 0.03*(pace-1)
	currentLoss := 0.22 + 0.10*difficulty - 0.03*(pace-1)
	switch errorType {
	case "conceptual", "logical":
		currentLoss += 0.05
	case "symbolic", "calculation":
		currentLoss += 0.02
	}

	next := prior
	if current.IsCorrect {
		next += numutil.ClampFloat(currentGain, 0.08, 0.3) * (1 - prior)
	} else {
		next -= numutil.ClampFloat(currentLoss, 0.12, 0.38) * prior
	}
	next += 0.10 * attentionSignal
	next = numutil.ClampFloat(next, 0.001, 0.999)

	effectiveAttempts := math.Max(float64(attemptCount), 0) + 1
	sequenceFactor := math.Log1p(float64(len(sequence))) / math.Log1p(dktMaxSequenceLength)
	confidence := numutil.ClampFloat(0.18+0.44*(1-math.Exp(-effectiveAttempts/5.0))+0.28*sequenceFactor+0.10*attentionWeight, 0, 1)
	return dktResult{
		mastery:         next,
		confidence:      confidence,
		attentionWeight: attentionWeight,
		sequenceLength:  len(sequence),
	}
}

func attentionSignal(targetConcept string, current LearningInteraction, sequence []LearningInteraction) (float64, float64) {
	if len(sequence) == 0 {
		return 0, 0
	}
	query := interactionEmbedding(current, len(sequence)-1, len(sequence))
	scores := make([]float64, len(sequence))
	maxScore := math.Inf(-1)
	for index, item := range sequence {
		key := interactionEmbedding(item, index, len(sequence))
		score := dot(query, key) / math.Sqrt(dktEmbeddingDim)
		scores[index] = score
		if score > maxScore {
			maxScore = score
		}
	}

	weightedSignal := 0.0
	maxAttention := 0.0
	denominator := 0.0
	for _, score := range scores {
		denominator += math.Exp(score - maxScore)
	}
	for index, item := range sequence {
		attention := math.Exp(scores[index]-maxScore) / math.Max(denominator, epsilon)
		if attention > maxAttention {
			maxAttention = attention
		}
		outcome := -1.0
		if item.IsCorrect {
			outcome = 1.0
		}
		relevance := conceptRelevance(targetConcept, current.ConceptIDs, item.ConceptIDs)
		difficultyWeight := 0.75 + 0.25*numutil.ClampFloat(item.Difficulty, 0, 1)
		weightedSignal += attention * outcome * relevance * difficultyWeight
	}
	return numutil.ClampFloat(weightedSignal, -1, 1), numutil.ClampFloat(maxAttention*float64(len(sequence)), 0, 1)
}

func conceptRelevance(targetConcept string, currentConcepts []string, itemConcepts []string) float64 {
	if containsString(itemConcepts, targetConcept) {
		return 1.0
	}
	for _, itemConcept := range itemConcepts {
		if containsString(currentConcepts, itemConcept) {
			return 0.55
		}
	}
	return 0.15
}

func interactionEmbedding(item LearningInteraction, position int, sequenceLength int) []float64 {
	vector := make([]float64, dktEmbeddingDim)
	addTokenEmbedding(vector, "exercise:"+item.ExerciseID, 1)
	for _, conceptID := range item.ConceptIDs {
		addTokenEmbedding(vector, "concept:"+conceptID, 0.85)
	}
	if item.IsCorrect {
		addTokenEmbedding(vector, "response:correct", 0.65)
	} else {
		addTokenEmbedding(vector, "response:incorrect", 0.65)
	}
	vector[dktEmbeddingDim-1] += numutil.ClampFloat(item.Difficulty, 0, 1) - 0.5

	pos := float64(position)
	if sequenceLength > 1 {
		pos = float64(position) / float64(sequenceLength-1) * float64(dktMaxSequenceLength-1)
	}
	for i := 0; i < dktEmbeddingDim; i++ {
		denominator := math.Pow(10000, float64(2*(i/2))/float64(dktEmbeddingDim))
		if i%2 == 0 {
			vector[i] += 0.25 * math.Sin(pos/denominator)
		} else {
			vector[i] += 0.25 * math.Cos(pos/denominator)
		}
	}
	normalizeVector(vector)
	return vector
}

func addTokenEmbedding(vector []float64, token string, weight float64) {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(token))
	value := hash.Sum64()
	index := int(value % uint64(len(vector)))
	sign := 1.0
	if (value>>8)&1 == 1 {
		sign = -1.0
	}
	vector[index] += sign * weight
	secondary := int((value >> 16) % uint64(len(vector)))
	vector[secondary] += sign * weight * 0.37
}

func normalizeVector(vector []float64) {
	norm := 0.0
	for _, value := range vector {
		norm += value * value
	}
	norm = math.Sqrt(norm)
	if norm <= epsilon {
		return
	}
	for i := range vector {
		vector[i] /= norm
	}
}

func dot(left []float64, right []float64) float64 {
	limit := len(left)
	if len(right) < limit {
		limit = len(right)
	}
	total := 0.0
	for i := 0; i < limit; i++ {
		total += left[i] * right[i]
	}
	return total
}

func applyForgetting(mastery float64, daysSinceLast float64, floor float64) float64 {
	if floor <= 0 {
		floor = dktRetentionFloor
	}
	if daysSinceLast <= 0 || mastery <= floor {
		return mastery
	}
	decayed := floor + (mastery-floor)*math.Exp(-0.025*daysSinceLast)
	return numutil.ClampFloat(decayed, 0.001, 0.999)
}
