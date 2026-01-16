package semantic

import (
	"strings"
)

// Rerank re-evaluates the similarity between the original prompt and the candidates.
// It returns the best match from the candidates based on a secondary similarity metric.
func Rerank(prompt string, candidates []RerankCandidate) *RerankCandidate {
	if len(candidates) == 0 {
		return nil
	}

	var bestMatch *RerankCandidate
	maxScore := -1.0

	for i := range candidates {
		score := CalculateStringSimilarity(prompt, candidates[i].Prompt)
		candidates[i].SecondaryScore = score

		// Final score could be a combination of vector score and secondary score
		// For simplicity, we use the secondary score if it's high enough, 
		// otherwise we can weight them.
		// Here we just pick the one with the highest secondary score among those 
		// that passed the initial vector threshold.
		if score > maxScore {
			maxScore = score
			bestMatch = &candidates[i]
		}
	}

	return bestMatch
}

// RerankCandidate represents a candidate for re-ranking.
type RerankCandidate struct {
	Prompt         string
	Response       string
	Model          string
	VectorScore    float64
	SecondaryScore float64
}

// CalculateStringSimilarity calculates Jaccard similarity between two strings based on words.
func CalculateStringSimilarity(s1, s2 string) float64 {
	s1 = strings.ToLower(strings.TrimSpace(s1))
	s2 = strings.ToLower(strings.TrimSpace(s2))

	if s1 == s2 {
		return 1.0
	}

	if s1 == "" || s2 == "" {
		return 0.0
	}

	w1 := strings.Fields(s1)
	w2 := strings.Fields(s2)

	set1 := make(map[string]struct{})
	for _, w := range w1 {
		set1[w] = struct{}{}
	}

	intersection := 0
	set2 := make(map[string]struct{})
	for _, w := range w2 {
		if _, ok := set2[w]; ok {
			continue
		}
		set2[w] = struct{}{}
		if _, ok := set1[w]; ok {
			intersection++
		}
	}

	union := len(set1) + len(set2) - intersection
	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}
