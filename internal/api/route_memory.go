package api

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	routeMemoryEpisodeLimit     = 256
	routeMemoryRecordLimit      = 120
	routeMemoryClusterThreshold = 0.50
)

type routeMemoryAccessStats struct {
	ConsultCount int
	ReuseCount   int
}

type routeMemoryCluster struct {
	ID       string
	Episodes []RouteMemoryRecord
}

func normalizeRouteMemoryEpisode(record RouteMemoryRecord) RouteMemoryRecord {
	now := time.Now()
	if record.ID == "" {
		record.ID = fmt.Sprintf("route-memory-%d", now.UnixNano())
	}
	record.Query = strings.TrimSpace(record.Query)
	record.Intent = strings.TrimSpace(record.Intent)
	record.SelectedAgentID = strings.TrimSpace(record.SelectedAgentID)
	record.SelectedModel = strings.TrimSpace(record.SelectedModel)
	record.SelectedPath = strings.TrimSpace(record.SelectedPath)
	record.RouteSource = strings.TrimSpace(record.RouteSource)
	record.AnswerPreview = truncateForPreview(record.AnswerPreview, 220)
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

func pruneRouteMemoryLocked(now time.Time) {
	filteredEpisodes := make([]RouteMemoryRecord, 0, len(conversationMemoryStore.episodes))
	for _, episode := range conversationMemoryStore.episodes {
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
	if len(filteredEpisodes) > routeMemoryEpisodeLimit {
		filteredEpisodes = filteredEpisodes[:routeMemoryEpisodeLimit]
	}
	conversationMemoryStore.episodes = append([]RouteMemoryRecord(nil), filteredEpisodes...)
}

func rebuildRouteMemoryRecordsLocked() []RouteMemoryRecord {
	clusters := clusterRouteMemoryEpisodes(conversationMemoryStore.episodes)
	records := make([]RouteMemoryRecord, 0, len(clusters))
	activeStats := make(map[string]routeMemoryAccessStats, len(clusters))
	for _, cluster := range clusters {
		stats := conversationMemoryStore.stats[cluster.ID]
		record := buildRouteMemoryRecordFromCluster(cluster, stats)
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
	if len(records) > routeMemoryRecordLimit {
		records = records[:routeMemoryRecordLimit]
	}
	trimmedStats := make(map[string]routeMemoryAccessStats, len(records))
	for _, record := range records {
		if stats, ok := activeStats[record.ID]; ok {
			trimmedStats[record.ID] = stats
		}
	}
	conversationMemoryStore.records = append([]RouteMemoryRecord(nil), records...)
	conversationMemoryStore.stats = trimmedStats
	return conversationMemoryStore.records
}

func clusterRouteMemoryEpisodes(episodes []RouteMemoryRecord) []routeMemoryCluster {
	if len(episodes) == 0 {
		return nil
	}
	items := append([]RouteMemoryRecord(nil), episodes...)
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	clusters := make([]routeMemoryCluster, 0, len(items))
	for _, episode := range items {
		bestIdx := -1
		bestScore := 0.0
		for i := range clusters {
			score := routeMemoryClusterSimilarity(clusters[i], episode)
			if score >= routeMemoryClusterThreshold && score > bestScore {
				bestIdx = i
				bestScore = score
			}
		}
		if bestIdx == -1 {
			clusters = append(clusters, routeMemoryCluster{
				ID:       episode.ID,
				Episodes: []RouteMemoryRecord{episode},
			})
			continue
		}
		clusters[bestIdx].Episodes = append(clusters[bestIdx].Episodes, episode)
		clusters[bestIdx].ID = stableRouteMemoryClusterID(clusters[bestIdx].Episodes)
	}
	return clusters
}

func stableRouteMemoryClusterID(episodes []RouteMemoryRecord) string {
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

func routeMemoryClusterSimilarity(cluster routeMemoryCluster, episode RouteMemoryRecord) float64 {
	best := 0.0
	for _, existing := range cluster.Episodes {
		score := routeMemoryEpisodeSimilarity(existing, episode)
		if score > best {
			best = score
		}
	}
	return best
}

func routeMemoryEpisodeSimilarity(a, b RouteMemoryRecord) float64 {
	if a.SelectedAgentID == "" || b.SelectedAgentID == "" || a.SelectedAgentID != b.SelectedAgentID {
		return 0
	}
	queryScore := tokenOverlapScore(tokenizeQuery(a.Query), tokenizeQuery(b.Query))
	previewScore := tokenOverlapScore(tokenizeQuery(a.AnswerPreview), tokenizeQuery(b.AnswerPreview))
	modelScore := 0.0
	if a.SelectedModel != "" && a.SelectedModel == b.SelectedModel {
		modelScore = 1
	}
	score := 0.55*queryScore + 0.20*previewScore + 0.15*modelScore
	if a.Intent != "" && a.Intent == b.Intent {
		score += 0.06
	}
	if a.RouteSource != "" && a.RouteSource == b.RouteSource {
		score += 0.04
	}
	return clamp01(score)
}

func buildRouteMemoryRecordFromCluster(cluster routeMemoryCluster, stats routeMemoryAccessStats) RouteMemoryRecord {
	episodes := append([]RouteMemoryRecord(nil), cluster.Episodes...)
	sort.SliceStable(episodes, func(i, j int) bool {
		return episodes[i].CreatedAt.After(episodes[j].CreatedAt)
	})
	latest := episodes[0]
	oldest := episodes[0]
	successCount := 0
	queries := make([]string, 0, len(episodes))
	previews := make([]string, 0, len(episodes))
	models := make([]string, 0, len(episodes))
	sources := make([]string, 0, len(episodes))
	paths := make([]string, 0, len(episodes))
	for _, episode := range episodes {
		if episode.CreatedAt.Before(oldest.CreatedAt) {
			oldest = episode
		}
		if episode.Succeeded {
			successCount++
		}
		if trimmed := strings.TrimSpace(episode.Query); trimmed != "" {
			queries = append(queries, trimmed)
		}
		if trimmed := strings.TrimSpace(episode.AnswerPreview); trimmed != "" {
			previews = append(previews, trimmed)
		}
		if trimmed := strings.TrimSpace(episode.SelectedModel); trimmed != "" {
			models = append(models, trimmed)
		}
		if trimmed := strings.TrimSpace(episode.RouteSource); trimmed != "" {
			sources = append(sources, trimmed)
		}
		if trimmed := strings.TrimSpace(episode.SelectedPath); trimmed != "" {
			paths = append(paths, trimmed)
		}
	}
	representativeQueries := distinctRecentStrings(queries, 3)
	keywords := buildRouteMemoryKeywords(queries, previews, latest.Intent, latest.SelectedAgentID, models)
	selectedModel := chooseDominantString(models, latest.SelectedModel)
	selectedPath := chooseDominantString(paths, latest.SelectedPath)
	routeSource := chooseDominantString(sources, latest.RouteSource)
	_, compressed, learnings := buildRouteMemoryCompression(latest.SelectedAgentID, latest.Intent, selectedModel, routeSource, keywords, len(episodes), successCount)
	stage := "episode"
	switch {
	case len(episodes) >= 4:
		stage = "insight"
	case len(episodes) >= 2:
		stage = "summary"
	}
	compressionRatio := compressionRatio(
		strings.Join(append(queries, previews...), " "),
		strings.Join(append([]string{compressed}, learnings...), " "),
	)
	record := RouteMemoryRecord{
		ID:                    cluster.ID,
		SessionID:             latest.SessionID,
		Query:                 chooseCanonicalTeamMemoryQuery(representativeQueries, latest.Query),
		Intent:                latest.Intent,
		SelectedAgentID:       latest.SelectedAgentID,
		SelectedModel:         selectedModel,
		SelectedPath:          selectedPath,
		RouteSource:           routeSource,
		AnswerPreview:         truncateForPreview(chooseDominantString(previews, latest.AnswerPreview), 220),
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
		OutcomeScore:          aggregateRouteMemoryOutcomeScore(episodes, successCount),
		CreatedAt:             latest.CreatedAt,
	}
	// Route memory keeps the compressed summary in both fields to avoid older UIs showing an empty preview.
	record.AnswerPreview = truncateForPreview(record.AnswerPreview, 220)
	if record.AnswerPreview == "" {
		record.AnswerPreview = compressed
	}
	return record
}

func aggregateRouteMemoryOutcomeScore(episodes []RouteMemoryRecord, successCount int) float64 {
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

func buildRouteMemoryCompression(agentID, intent, model, routeSource string, keywords []string, sourceCount, successCount int) (string, string, []string) {
	intentLabel := strings.TrimSpace(intent)
	if intentLabel == "" {
		intentLabel = "general"
	}
	focus := "broad follow-up tasks"
	if len(keywords) > 0 {
		focus = strings.Join(keywords[:minInt(len(keywords), 4)], ", ")
	}
	sourceLabel := routeSource
	if sourceLabel == "" {
		sourceLabel = "score-router"
	}
	modelLabel := model
	if modelLabel == "" {
		modelLabel = "default-model"
	}
	compressed := fmt.Sprintf(
		"For %s prompts around %s, prefer agent=%s model=%s via %s. Evidence=%d episode(s), success=%d.",
		intentLabel,
		focus,
		agentID,
		modelLabel,
		sourceLabel,
		sourceCount,
		successCount,
	)
	summary := compressed
	if sourceCount > 1 {
		summary += " This is a compressed route-memory synthesis, not a single turn."
	}
	learnings := []string{
		fmt.Sprintf("Route %s prompts mentioning %s to %s.", intentLabel, focus, agentID),
		fmt.Sprintf("Prefer model %s and path source %s for similar follow-ups.", modelLabel, sourceLabel),
	}
	if sourceCount >= 3 {
		learnings = append(learnings, fmt.Sprintf("Trust this record more than a one-off route because it aggregates %d related turns.", sourceCount))
	}
	return truncateForPreview(summary, 320), truncateForPreview(compressed, 220), cleanStringList(learnings)
}

func buildRouteMemoryKeywords(queries, previews []string, intent, agentID string, models []string) []string {
	tokens := make([]string, 0, len(queries)+len(previews)+len(models)+2)
	for _, query := range queries {
		tokens = append(tokens, memoryLexemes(query)...)
	}
	for _, preview := range previews {
		tokens = append(tokens, memoryLexemes(preview)...)
	}
	tokens = append(tokens, memoryLexemes(intent)...)
	tokens = append(tokens, memoryLexemes(agentID)...)
	tokens = append(tokens, models...)
	return topStringsByFrequency(tokens, 8)
}

func chooseDominantString(items []string, fallback string) string {
	ranked := topStringsByFrequency(items, 1)
	if len(ranked) > 0 {
		return ranked[0]
	}
	return strings.TrimSpace(fallback)
}

func noteRouteMemoryConsulted(records []RouteMemoryRecord) {
	if len(records) == 0 {
		return
	}
	conversationMemoryStore.Lock()
	defer conversationMemoryStore.Unlock()
	for _, record := range records {
		stats := conversationMemoryStore.stats[record.ID]
		stats.ConsultCount++
		conversationMemoryStore.stats[record.ID] = stats
		for i := range conversationMemoryStore.records {
			if conversationMemoryStore.records[i].ID == record.ID {
				conversationMemoryStore.records[i].ConsultCount = stats.ConsultCount
				break
			}
		}
	}
}

func noteRouteMemoryReused(record *RouteMemoryRecord) {
	if record == nil || record.ID == "" {
		return
	}
	conversationMemoryStore.Lock()
	defer conversationMemoryStore.Unlock()
	stats := conversationMemoryStore.stats[record.ID]
	stats.ReuseCount++
	conversationMemoryStore.stats[record.ID] = stats
	for i := range conversationMemoryStore.records {
		if conversationMemoryStore.records[i].ID == record.ID {
			conversationMemoryStore.records[i].ReuseCount = stats.ReuseCount
			break
		}
	}
}
