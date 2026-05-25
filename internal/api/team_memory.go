package api

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	teamMemoryEpisodeLimit     = 256
	teamMemoryRecordLimit      = 120
	teamMemoryClusterThreshold = 0.48
)

type teamMemoryAccessStats struct {
	ConsultCount int
	ReuseCount   int
}

type teamMemoryCluster struct {
	ID       string
	Episodes []TeamMemoryRecord
}

func normalizeTeamMemoryEpisode(record TeamMemoryRecord) TeamMemoryRecord {
	now := time.Now()
	if record.ID == "" {
		record.ID = fmt.Sprintf("team-memory-%d", now.UnixNano())
	}
	record.Query = strings.TrimSpace(record.Query)
	record.Intent = strings.TrimSpace(record.Intent)
	record.SelectedTeamID = strings.TrimSpace(record.SelectedTeamID)
	record.SelectedAgentIDs = cleanStringList(record.SelectedAgentIDs)
	record.Summary = truncateForPreview(record.Summary, 280)
	record.CompressedSummary = ""
	record.DistilledLearnings = nil
	record.RepresentativeQueries = nil
	record.RetrievalHints = nil
	record.MemoryStage = "episode"
	record.SourceCount = 1
	if record.SuccessCount <= 0 {
		if record.Succeeded {
			record.SuccessCount = 1
		} else {
			record.SuccessCount = 0
		}
	}
	record.ConsultCount = 0
	record.ReuseCount = 0
	record.CompressionRatio = 1
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	if record.FirstRecordedAt.IsZero() {
		record.FirstRecordedAt = record.CreatedAt
	}
	if record.LastReinforcedAt.IsZero() {
		record.LastReinforcedAt = record.CreatedAt
	}
	return record
}

func pruneTeamMemoryLocked(now time.Time) {
	filteredEpisodes := make([]TeamMemoryRecord, 0, len(teamMemoryStore.episodes))
	for _, episode := range teamMemoryStore.episodes {
		last := episode.LastReinforcedAt
		if last.IsZero() {
			last = episode.CreatedAt
		}
		if now.Sub(last) > memoryTTL(episode.OutcomeScore) {
			continue
		}
		filteredEpisodes = append(filteredEpisodes, episode)
	}
	sort.SliceStable(filteredEpisodes, func(i, j int) bool {
		return filteredEpisodes[i].CreatedAt.After(filteredEpisodes[j].CreatedAt)
	})
	if len(filteredEpisodes) > teamMemoryEpisodeLimit {
		filteredEpisodes = filteredEpisodes[:teamMemoryEpisodeLimit]
	}
	teamMemoryStore.episodes = append([]TeamMemoryRecord(nil), filteredEpisodes...)
}

func rebuildTeamMemoryRecordsLocked() []TeamMemoryRecord {
	clusters := clusterTeamMemoryEpisodes(teamMemoryStore.episodes)
	records := make([]TeamMemoryRecord, 0, len(clusters))
	activeStats := make(map[string]teamMemoryAccessStats, len(clusters))
	for _, cluster := range clusters {
		stats := teamMemoryStore.stats[cluster.ID]
		record := buildTeamMemoryRecordFromCluster(cluster, stats)
		records = append(records, record)
		activeStats[cluster.ID] = stats
	}
	sort.SliceStable(records, func(i, j int) bool {
		if records[i].LastReinforcedAt.Equal(records[j].LastReinforcedAt) {
			if records[i].ReuseCount == records[j].ReuseCount {
				return records[i].OutcomeScore > records[j].OutcomeScore
			}
			return records[i].ReuseCount > records[j].ReuseCount
		}
		return records[i].LastReinforcedAt.After(records[j].LastReinforcedAt)
	})
	if len(records) > teamMemoryRecordLimit {
		records = records[:teamMemoryRecordLimit]
	}
	trimmedStats := make(map[string]teamMemoryAccessStats, len(records))
	for _, record := range records {
		if stats, ok := activeStats[record.ID]; ok {
			trimmedStats[record.ID] = stats
		}
	}
	teamMemoryStore.records = append([]TeamMemoryRecord(nil), records...)
	teamMemoryStore.stats = trimmedStats
	return teamMemoryStore.records
}

func clusterTeamMemoryEpisodes(episodes []TeamMemoryRecord) []teamMemoryCluster {
	if len(episodes) == 0 {
		return nil
	}
	items := append([]TeamMemoryRecord(nil), episodes...)
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	clusters := make([]teamMemoryCluster, 0, len(items))
	for _, episode := range items {
		bestIdx := -1
		bestScore := 0.0
		for i := range clusters {
			score := teamMemoryClusterSimilarity(clusters[i], episode)
			if score >= teamMemoryClusterThreshold && score > bestScore {
				bestIdx = i
				bestScore = score
			}
		}
		if bestIdx == -1 {
			clusters = append(clusters, teamMemoryCluster{
				ID:       episode.ID,
				Episodes: []TeamMemoryRecord{episode},
			})
			continue
		}
		clusters[bestIdx].Episodes = append(clusters[bestIdx].Episodes, episode)
		clusters[bestIdx].ID = stableTeamMemoryClusterID(clusters[bestIdx].Episodes)
	}
	return clusters
}

func stableTeamMemoryClusterID(episodes []TeamMemoryRecord) string {
	if len(episodes) == 0 {
		return ""
	}
	oldest := episodes[0]
	for _, episode := range episodes[1:] {
		if episode.CreatedAt.Before(oldest.CreatedAt) {
			oldest = episode
		}
	}
	return oldest.ID
}

func teamMemoryClusterSimilarity(cluster teamMemoryCluster, episode TeamMemoryRecord) float64 {
	best := 0.0
	for _, existing := range cluster.Episodes {
		score := teamMemoryEpisodeSimilarity(existing, episode)
		if score > best {
			best = score
		}
	}
	return best
}

func teamMemoryEpisodeSimilarity(a, b TeamMemoryRecord) float64 {
	if a.SelectedTeamID == "" || b.SelectedTeamID == "" || a.SelectedTeamID != b.SelectedTeamID {
		return 0
	}
	queryScore := tokenOverlapScore(tokenizeQuery(a.Query), tokenizeQuery(b.Query))
	summaryScore := tokenOverlapScore(tokenizeQuery(a.Summary), tokenizeQuery(b.Summary))
	agentScore := stringOverlapScore(a.SelectedAgentIDs, b.SelectedAgentIDs)
	score := 0.55*queryScore + 0.20*summaryScore + 0.15*agentScore
	if a.Intent != "" && a.Intent == b.Intent {
		score += 0.10
	}
	return clamp01(score)
}

func stringOverlapScore(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	set := make(map[string]struct{}, len(b))
	for _, item := range b {
		set[strings.ToLower(strings.TrimSpace(item))] = struct{}{}
	}
	matches := 0
	for _, item := range a {
		if _, ok := set[strings.ToLower(strings.TrimSpace(item))]; ok {
			matches++
		}
	}
	denominator := len(a)
	if len(b) > denominator {
		denominator = len(b)
	}
	return float64(matches) / float64(denominator)
}

func buildTeamMemoryRecordFromCluster(cluster teamMemoryCluster, stats teamMemoryAccessStats) TeamMemoryRecord {
	episodes := append([]TeamMemoryRecord(nil), cluster.Episodes...)
	sort.SliceStable(episodes, func(i, j int) bool {
		return episodes[i].CreatedAt.After(episodes[j].CreatedAt)
	})
	latest := episodes[0]
	oldest := episodes[0]
	successCount := 0
	allAgents := make([]string, 0, len(episodes)*2)
	summaryTexts := make([]string, 0, len(episodes))
	queries := make([]string, 0, len(episodes))
	for _, episode := range episodes {
		if episode.CreatedAt.Before(oldest.CreatedAt) {
			oldest = episode
		}
		if episode.Succeeded {
			successCount++
		}
		allAgents = append(allAgents, episode.SelectedAgentIDs...)
		if trimmed := strings.TrimSpace(episode.Summary); trimmed != "" {
			summaryTexts = append(summaryTexts, trimmed)
		}
		if trimmed := strings.TrimSpace(episode.Query); trimmed != "" {
			queries = append(queries, trimmed)
		}
	}
	selectedAgents := topStringsByFrequency(allAgents, 4)
	representativeQueries := distinctRecentStrings(queries, 3)
	keywords := buildTeamMemoryKeywords(queries, summaryTexts, latest.Intent, selectedAgents)
	summary, compressed, learnings := buildTeamMemoryCompression(latest.SelectedTeamID, latest.Intent, selectedAgents, keywords, len(episodes), successCount)
	stage := "episode"
	switch {
	case len(episodes) >= 4:
		stage = "insight"
	case len(episodes) >= 2:
		stage = "summary"
	}
	compressionRatio := compressionRatio(
		strings.Join(append(queries, summaryTexts...), " "),
		strings.Join(append([]string{compressed}, learnings...), " "),
	)
	record := TeamMemoryRecord{
		ID:                    cluster.ID,
		SessionID:             latest.SessionID,
		Query:                 chooseCanonicalTeamMemoryQuery(representativeQueries, latest.Query),
		Intent:                latest.Intent,
		SelectedTeamID:        latest.SelectedTeamID,
		SelectedAgentIDs:      selectedAgents,
		Summary:               summary,
		CompressedSummary:     compressed,
		DistilledLearnings:    learnings,
		RepresentativeQueries: representativeQueries,
		RetrievalHints:        keywords,
		MemoryStage:           stage,
		SourceCount:           len(episodes),
		SuccessCount:          successCount,
		ConsultCount:          stats.ConsultCount,
		ReuseCount:            stats.ReuseCount,
		CompressionRatio:      compressionRatio,
		FirstRecordedAt:       oldest.CreatedAt,
		LastReinforcedAt:      latest.CreatedAt,
		Succeeded:             successCount > 0,
		OutcomeScore:          aggregateTeamMemoryOutcomeScore(episodes, successCount),
		CreatedAt:             latest.CreatedAt,
	}
	return record
}

func chooseCanonicalTeamMemoryQuery(queries []string, fallback string) string {
	if len(queries) == 0 {
		return strings.TrimSpace(fallback)
	}
	best := queries[0]
	for _, query := range queries[1:] {
		if len([]rune(query)) < len([]rune(best)) {
			best = query
		}
	}
	return best
}

func aggregateTeamMemoryOutcomeScore(episodes []TeamMemoryRecord, successCount int) float64 {
	if len(episodes) == 0 {
		return 0.4
	}
	totalWeight := 0.0
	weighted := 0.0
	for idx, episode := range episodes {
		weight := float64(len(episodes) - idx)
		if episode.Succeeded {
			weight += 0.5
		}
		totalWeight += weight
		weighted += episode.OutcomeScore * weight
	}
	score := 0.4
	if totalWeight > 0 {
		score = weighted / totalWeight
	}
	successRate := float64(successCount) / float64(len(episodes))
	score = 0.75*score + 0.25*(0.45+0.45*successRate)
	if len(episodes) >= 3 && successRate >= 0.8 {
		score += 0.03
	}
	return clamp01(score)
}

func buildTeamMemoryCompression(teamID, intent string, selectedAgents, keywords []string, sourceCount, successCount int) (string, string, []string) {
	intentLabel := strings.TrimSpace(intent)
	if intentLabel == "" {
		intentLabel = "general"
	}
	focus := "broad collaboration"
	if len(keywords) > 0 {
		focus = strings.Join(keywords[:minInt(len(keywords), 4)], ", ")
	}
	lead := "no fixed lead"
	if len(selectedAgents) > 0 {
		lead = selectedAgents[0]
	}
	compressed := fmt.Sprintf(
		"%s memory for %s tasks around %s. Preferred lead=%s. Evidence=%d episode(s), success=%d.",
		teamID,
		intentLabel,
		focus,
		lead,
		sourceCount,
		successCount,
	)
	if len(selectedAgents) > 1 {
		compressed += fmt.Sprintf(" Supporting agents=%s.", strings.Join(selectedAgents[1:], ", "))
	}
	summary := compressed
	if sourceCount > 1 {
		summary += fmt.Sprintf(" This record is a compressed synthesis rather than a single-run note.")
	}
	learnings := []string{
		fmt.Sprintf("Route %s work to %s when the prompt mentions %s.", intentLabel, teamID, focus),
		fmt.Sprintf("Start with %s as the lead agent and reuse the rest as supporting specialists.", lead),
	}
	if sourceCount >= 3 {
		learnings = append(learnings, fmt.Sprintf("Prefer this distilled memory over one-off episodes because it aggregates %d separate runs.", sourceCount))
	}
	return truncateForPreview(summary, 320), truncateForPreview(compressed, 220), cleanStringList(learnings)
}

func buildTeamMemoryKeywords(queries, summaries []string, intent string, selectedAgents []string) []string {
	tokens := make([]string, 0, len(queries)+len(summaries)+len(selectedAgents)+1)
	for _, query := range queries {
		tokens = append(tokens, memoryLexemes(query)...)
	}
	for _, summary := range summaries {
		tokens = append(tokens, memoryLexemes(summary)...)
	}
	tokens = append(tokens, memoryLexemes(intent)...)
	tokens = append(tokens, selectedAgents...)
	return topStringsByFrequency(tokens, 8)
}

func memoryLexemes(value string) []string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return nil
	}
	replacer := strings.NewReplacer(",", " ", ".", " ", "?", " ", "!", " ", ":", " ", ";", " ", "(", " ", ")", " ", "\n", " ", "\t", " ")
	parts := strings.Fields(replacer.Replace(value))
	seen := map[string]struct{}{}
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || isMemoryNoiseToken(part) {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		result = append(result, part)
	}
	return result
}

func isMemoryNoiseToken(token string) bool {
	if len([]rune(token)) <= 1 {
		return true
	}
	switch token {
	case "the", "and", "for", "with", "that", "this", "from", "into", "about", "then", "than", "were", "when", "what", "how", "why", "which", "please", "help", "team", "agent", "work", "task":
		return true
	}
	return false
}

func topStringsByFrequency(items []string, limit int) []string {
	if len(items) == 0 || limit <= 0 {
		return nil
	}
	counts := map[string]int{}
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		counts[trimmed]++
	}
	type candidate struct {
		Value string
		Count int
	}
	candidates := make([]candidate, 0, len(counts))
	for value, count := range counts {
		candidates = append(candidates, candidate{Value: value, Count: count})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Count == candidates[j].Count {
			return candidates[i].Value < candidates[j].Value
		}
		return candidates[i].Count > candidates[j].Count
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	result := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		result = append(result, candidate.Value)
	}
	return result
}

func distinctRecentStrings(items []string, limit int) []string {
	if len(items) == 0 || limit <= 0 {
		return nil
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, minInt(len(items), limit))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
		if len(result) >= limit {
			break
		}
	}
	return result
}

func compressionRatio(raw, compressed string) float64 {
	raw = strings.TrimSpace(raw)
	compressed = strings.TrimSpace(compressed)
	if raw == "" || compressed == "" {
		return 1
	}
	ratio := float64(len(compressed)) / float64(len(raw))
	if ratio < 0 {
		return 0
	}
	if ratio > 1 {
		return 1
	}
	return ratio
}

func noteTeamMemoryConsulted(records []TeamMemoryRecord) {
	if len(records) == 0 {
		return
	}
	teamMemoryStore.Lock()
	defer teamMemoryStore.Unlock()
	for _, record := range records {
		stats := teamMemoryStore.stats[record.ID]
		stats.ConsultCount++
		teamMemoryStore.stats[record.ID] = stats
		for i := range teamMemoryStore.records {
			if teamMemoryStore.records[i].ID == record.ID {
				teamMemoryStore.records[i].ConsultCount = stats.ConsultCount
				break
			}
		}
	}
}

func noteTeamMemoryReused(record *TeamMemoryRecord) {
	if record == nil || record.ID == "" {
		return
	}
	teamMemoryStore.Lock()
	defer teamMemoryStore.Unlock()
	stats := teamMemoryStore.stats[record.ID]
	stats.ReuseCount++
	teamMemoryStore.stats[record.ID] = stats
	for i := range teamMemoryStore.records {
		if teamMemoryStore.records[i].ID == record.ID {
			teamMemoryStore.records[i].ReuseCount = stats.ReuseCount
			break
		}
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
