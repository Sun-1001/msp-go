package llmrouting

import (
	cryptorand "crypto/rand"
	"errors"
	"math/big"
	"sort"
)

// Candidate associates one routable value with channel scheduling metadata.
type Candidate[T any] struct {
	Value    T
	Priority int
	Weight   int
}

// Intn returns a value in [0, max). It is injectable so scheduling stays testable.
type Intn func(max int) (int, error)

// CryptoIntn selects an unbiased integer with crypto/rand.
func CryptoIntn(max int) (int, error) {
	if max <= 0 {
		return 0, errors.New("random upper bound must be positive")
	}
	value, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(value.Int64()), nil
}

// Order returns candidates by descending priority and weighted random order
// within each priority. Each candidate appears exactly once.
func Order[T any](candidates []Candidate[T], intn Intn) ([]T, error) {
	if len(candidates) == 0 {
		return []T{}, nil
	}
	if intn == nil {
		intn = CryptoIntn
	}

	groups := make(map[int][]Candidate[T])
	priorities := make([]int, 0)
	for _, candidate := range candidates {
		if _, exists := groups[candidate.Priority]; !exists {
			priorities = append(priorities, candidate.Priority)
		}
		groups[candidate.Priority] = append(groups[candidate.Priority], candidate)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(priorities)))

	ordered := make([]T, 0, len(candidates))
	for _, priority := range priorities {
		group, err := weightedPermutation(groups[priority], intn)
		if err != nil {
			return nil, err
		}
		ordered = append(ordered, group...)
	}
	return ordered, nil
}

func weightedPermutation[T any](candidates []Candidate[T], intn Intn) ([]T, error) {
	remaining := append([]Candidate[T](nil), candidates...)
	ordered := make([]T, 0, len(remaining))
	for len(remaining) > 0 {
		total := 0
		for _, candidate := range remaining {
			total += max(candidate.Weight, 1)
		}
		pick, err := intn(total)
		if err != nil {
			return nil, err
		}
		selected := 0
		for index, candidate := range remaining {
			pick -= max(candidate.Weight, 1)
			if pick < 0 {
				selected = index
				break
			}
		}
		ordered = append(ordered, remaining[selected].Value)
		remaining = append(remaining[:selected], remaining[selected+1:]...)
	}
	return ordered, nil
}
