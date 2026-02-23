// Package pipeline orchestrates session loading, caching, and metric aggregation.
package pipeline

import (
	"sort"
	"strings"
	"time"

	"cburn/internal/config"
	"cburn/internal/model"
)

// Aggregate computes summary statistics from a slice of session stats,
// filtered to sessions within the given time range.
func Aggregate(sessions []model.SessionStats, since, until time.Time) model.SummaryStats {
	filtered := FilterByTime(sessions, since, until)

	var stats model.SummaryStats
	activeDays := make(map[string]struct{})

	for _, s := range filtered {
		stats.TotalSessions++
		stats.TotalPrompts += s.UserMessages
		stats.TotalAPICalls += s.APICalls
		stats.TotalDurationSecs += s.DurationSecs

		stats.InputTokens += s.InputTokens
		stats.OutputTokens += s.OutputTokens
		stats.CacheCreation5mTokens += s.CacheCreation5mTokens
		stats.CacheCreation1hTokens += s.CacheCreation1hTokens
		stats.CacheReadTokens += s.CacheReadTokens
		stats.EstimatedCost += s.EstimatedCost

		if !s.StartTime.IsZero() {
			day := s.StartTime.Local().Format("2006-01-02")
			activeDays[day] = struct{}{}
		}
	}

	stats.ActiveDays = len(activeDays)
	stats.TotalBilledTokens = stats.InputTokens + stats.OutputTokens +
		stats.CacheCreation5mTokens + stats.CacheCreation1hTokens

	// Cache hit rate
	totalCacheInput := stats.CacheReadTokens + stats.CacheCreation5mTokens +
		stats.CacheCreation1hTokens + stats.InputTokens
	if totalCacheInput > 0 {
		stats.CacheHitRate = float64(stats.CacheReadTokens) / float64(totalCacheInput)
	}

	// Cache savings (sum across all models found in sessions)
	for _, s := range filtered {
		for modelName, mu := range s.Models {
			stats.CacheSavings += config.CalculateCacheSavings(modelName, mu.CacheReadTokens)
		}
	}

	// Per-active-day rates
	if stats.ActiveDays > 0 {
		days := float64(stats.ActiveDays)
		stats.CostPerDay = stats.EstimatedCost / days
		stats.TokensPerDay = int64(float64(stats.TotalBilledTokens) / days)
		stats.SessionsPerDay = float64(stats.TotalSessions) / days
		stats.PromptsPerDay = float64(stats.TotalPrompts) / days
		stats.MinutesPerDay = float64(stats.TotalDurationSecs) / 60 / days
	}

	return stats
}

// AggregateDays computes per-day statistics from sessions.
func AggregateDays(sessions []model.SessionStats, since, until time.Time) []model.DailyStats {
	filtered := FilterByTime(sessions, since, until)

	dayMap := make(map[string]*model.DailyStats)

	for _, s := range filtered {
		if s.StartTime.IsZero() {
			continue
		}
		dayKey := s.StartTime.Local().Format("2006-01-02")
		ds, ok := dayMap[dayKey]
		if !ok {
			t, _ := time.ParseInLocation("2006-01-02", dayKey, time.Local)
			ds = &model.DailyStats{Date: t}
			dayMap[dayKey] = ds
		}

		ds.Sessions++
		ds.Prompts += s.UserMessages
		ds.APICalls += s.APICalls
		ds.DurationSecs += s.DurationSecs
		ds.InputTokens += s.InputTokens
		ds.OutputTokens += s.OutputTokens
		ds.CacheCreation5m += s.CacheCreation5mTokens
		ds.CacheCreation1h += s.CacheCreation1hTokens
		ds.CacheReadTokens += s.CacheReadTokens
		ds.EstimatedCost += s.EstimatedCost
	}

	// Fill in every day in the range so the chart shows gaps as zeros
	day := since.Local().Truncate(24 * time.Hour)
	end := until.Local().Truncate(24 * time.Hour)
	for !day.After(end) {
		dayKey := day.Format("2006-01-02")
		if _, ok := dayMap[dayKey]; !ok {
			dayMap[dayKey] = &model.DailyStats{Date: day}
		}
		day = day.AddDate(0, 0, 1)
	}

	// Convert to sorted slice (most recent first)
	days := make([]model.DailyStats, 0, len(dayMap))
	for _, ds := range dayMap {
		days = append(days, *ds)
	}
	sort.Slice(days, func(i, j int) bool {
		return days[i].Date.After(days[j].Date)
	})

	return days
}

// AggregateModels computes per-model statistics from sessions.
func AggregateModels(sessions []model.SessionStats, since, until time.Time) []model.ModelStats {
	filtered := FilterByTime(sessions, since, until)

	modelMap := make(map[string]*model.ModelStats)
	totalCalls := 0

	for _, s := range filtered {
		for modelName, mu := range s.Models {
			ms, ok := modelMap[modelName]
			if !ok {
				ms = &model.ModelStats{Model: modelName}
				modelMap[modelName] = ms
			}
			ms.APICalls += mu.APICalls
			ms.InputTokens += mu.InputTokens
			ms.OutputTokens += mu.OutputTokens
			ms.CacheCreation5m += mu.CacheCreation5mTokens
			ms.CacheCreation1h += mu.CacheCreation1hTokens
			ms.CacheReadTokens += mu.CacheReadTokens
			ms.EstimatedCost += mu.EstimatedCost
			totalCalls += mu.APICalls
		}
	}

	// Compute share percentages and sort by cost descending
	models := make([]model.ModelStats, 0, len(modelMap))
	for _, ms := range modelMap {
		if totalCalls > 0 {
			ms.SharePercent = float64(ms.APICalls) / float64(totalCalls) * 100
		}
		models = append(models, *ms)
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].EstimatedCost > models[j].EstimatedCost
	})

	return models
}

// AggregateProjects computes per-project statistics from sessions.
func AggregateProjects(sessions []model.SessionStats, since, until time.Time) []model.ProjectStats {
	filtered := FilterByTime(sessions, since, until)

	projMap := make(map[string]*model.ProjectStats)

	for _, s := range filtered {
		ps, ok := projMap[s.Project]
		if !ok {
			ps = &model.ProjectStats{Project: s.Project}
			projMap[s.Project] = ps
		}
		ps.Sessions++
		ps.Prompts += s.UserMessages
		ps.TotalTokens += s.InputTokens + s.OutputTokens +
			s.CacheCreation5mTokens + s.CacheCreation1hTokens
		ps.EstimatedCost += s.EstimatedCost
	}

	// Sort by cost descending
	projects := make([]model.ProjectStats, 0, len(projMap))
	for _, ps := range projMap {
		projects = append(projects, *ps)
	}
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].EstimatedCost > projects[j].EstimatedCost
	})

	return projects
}

// AggregateHourly computes prompt counts by hour of day.
func AggregateHourly(sessions []model.SessionStats, since, until time.Time) []model.HourlyStats {
	filtered := FilterByTime(sessions, since, until)

	hours := make([]model.HourlyStats, 24)
	for i := range hours {
		hours[i].Hour = i
	}

	// We attribute all prompts and tokens to the session's start hour
	for _, s := range filtered {
		if s.StartTime.IsZero() {
			continue
		}
		h := s.StartTime.Local().Hour()
		hours[h].Prompts += s.UserMessages
		hours[h].Sessions++
		hours[h].Tokens += s.InputTokens + s.OutputTokens
	}

	return hours
}

// FilterByTime returns sessions whose start time falls within [since, until).
func FilterByTime(sessions []model.SessionStats, since, until time.Time) []model.SessionStats {
	if since.IsZero() && until.IsZero() {
		return sessions
	}

	var result []model.SessionStats
	for _, s := range sessions {
		if s.StartTime.IsZero() {
			continue
		}
		if !since.IsZero() && s.StartTime.Before(since) {
			continue
		}
		if !until.IsZero() && !s.StartTime.Before(until) {
			continue
		}
		result = append(result, s)
	}
	return result
}

// FilterByProject returns sessions matching the project substring.
func FilterByProject(sessions []model.SessionStats, project string) []model.SessionStats {
	if project == "" {
		return sessions
	}
	var result []model.SessionStats
	for _, s := range sessions {
		if containsIgnoreCase(s.Project, project) {
			result = append(result, s)
		}
	}
	return result
}

// FilterByModel returns sessions that have at least one API call to the given model.
func FilterByModel(sessions []model.SessionStats, modelFilter string) []model.SessionStats {
	if modelFilter == "" {
		return sessions
	}
	var result []model.SessionStats
	for _, s := range sessions {
		for m := range s.Models {
			if containsIgnoreCase(m, modelFilter) {
				result = append(result, s)
				break
			}
		}
	}
	return result
}

func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// AggregateTodayHourly computes 24 hourly token buckets for today (local time).
func AggregateTodayHourly(sessions []model.SessionStats) []model.HourlyStats {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	todayEnd := todayStart.Add(24 * time.Hour)

	hours := make([]model.HourlyStats, 24)
	for i := range hours {
		hours[i].Hour = i
	}

	for _, s := range sessions {
		if s.StartTime.IsZero() {
			continue
		}
		local := s.StartTime.Local()
		if local.Before(todayStart) || !local.Before(todayEnd) {
			continue
		}
		h := local.Hour()
		hours[h].Prompts += s.UserMessages
		hours[h].Sessions++
		hours[h].Tokens += s.InputTokens + s.OutputTokens
	}
	return hours
}

// AggregateLastHour computes 12 five-minute token buckets for the last 60 minutes.
func AggregateLastHour(sessions []model.SessionStats) []model.MinuteStats {
	now := time.Now()
	hourAgo := now.Add(-1 * time.Hour)

	buckets := make([]model.MinuteStats, 12)
	for i := range buckets {
		buckets[i].Minute = i
	}

	for _, s := range sessions {
		if s.StartTime.IsZero() {
			continue
		}
		local := s.StartTime.Local()
		if local.Before(hourAgo) || !local.Before(now) {
			continue
		}
		// Compute which 5-minute bucket (0-11) this falls into
		minutesAgo := int(now.Sub(local).Minutes())
		bucketIdx := 11 - (minutesAgo / 5) // 11 = most recent, 0 = oldest
		if bucketIdx < 0 {
			bucketIdx = 0
		}
		if bucketIdx > 11 {
			bucketIdx = 11
		}
		buckets[bucketIdx].Tokens += s.InputTokens + s.OutputTokens
	}
	return buckets
}
