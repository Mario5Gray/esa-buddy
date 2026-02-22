// Package main implements usage statistics collection and display functionality
// for the ESA command-line tool. This module provides structured data collection
// and formatted output of user interaction patterns.
package conversation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/meain/esa/internal/conversation/history"
	"github.com/meain/esa/internal/utils"
)

// Statistics data structures
type DayStats struct {
	Count    int
	Tokens   int
	Duration time.Duration
}

type HourStats struct {
	Count int
}

type AgentStats struct {
	Count    int
	Tokens   int
	Duration time.Duration
}

type ModelStats struct {
	Count    int
	Tokens   int
	Duration time.Duration
}

// StatsCollector collects and processes usage statistics
type StatsCollector struct {
	dayStats           map[string]DayStats
	hourStats          map[int]HourStats
	agentStats         map[string]AgentStats
	modelStats         map[string]ModelStats
	totalConversations int
	totalTokens        int // sum of all tracked tokens across conversations
}

// NewStatsCollector creates a new statistics collector
func NewStatsCollector() *StatsCollector {
	return &StatsCollector{
		dayStats:   make(map[string]DayStats),
		hourStats:  make(map[int]HourStats),
		agentStats: make(map[string]AgentStats),
		modelStats: make(map[string]ModelStats),
	}
}

// ProcessHistoryFile processes a single history file and updates statistics
func (sc *StatsCollector) ProcessHistoryFile(filePath string, fileName string, fileModTime time.Time) error {
	historyData, err := os.ReadFile(filePath)
	if err != nil {
		return utils.WrapFileError("read", filePath, err)
	}

	var history history.ConversationHistory
	if err := json.Unmarshal(historyData, &history); err != nil {
		// Skip files with JSON parsing errors silently
		return nil
	}

	// Extract date and hour information
	dateKey := fileModTime.Format("2006-01-02")
	hourKey := fileModTime.Hour()

	// Extract token count (zero if history predates token tracking)
	tokens := 0
	if history.Usage != nil {
		tokens = history.Usage.TotalTokens
	}

	// Update statistics
	sc.updateDayStats(dateKey, tokens)
	sc.updateHourStats(hourKey)
	sc.updateAgentStats(history.AgentPath, tokens)
	sc.updateModelStats(history.Model, tokens)
	sc.totalConversations++
	sc.totalTokens += tokens

	return nil
}

// updateDayStats updates daily usage statistics
func (sc *StatsCollector) updateDayStats(dateKey string, tokens int) {
	dayStat := sc.dayStats[dateKey]
	dayStat.Count++
	dayStat.Tokens += tokens
	sc.dayStats[dateKey] = dayStat
}

// updateHourStats updates hourly usage statistics
func (sc *StatsCollector) updateHourStats(hourKey int) {
	hourStat := sc.hourStats[hourKey]
	hourStat.Count++
	sc.hourStats[hourKey] = hourStat
}

// updateAgentStats updates agent usage statistics
func (sc *StatsCollector) updateAgentStats(agentPath string, tokens int) {
	if agentPath == "" {
		return
	}

	// Extract agent name from path
	var agentName string
	if strings.HasPrefix(agentPath, "builtin:") {
		agentName = strings.TrimPrefix(agentPath, "builtin:")
	} else {
		agentName = strings.TrimSuffix(filepath.Base(agentPath), ".toml")
	}

	agentStat := sc.agentStats[agentName]
	agentStat.Count++
	agentStat.Tokens += tokens
	sc.agentStats[agentName] = agentStat
}

// updateModelStats updates model usage statistics
func (sc *StatsCollector) updateModelStats(model string, tokens int) {
	if model == "" {
		return
	}

	modelStat := sc.modelStats[model]
	modelStat.Count++
	modelStat.Tokens += tokens
	sc.modelStats[model] = modelStat
}

// PrintStatistics prints formatted usage statistics
func (sc *StatsCollector) PrintStatistics(showAll bool) {
	headerStyle := color.New(color.FgHiCyan, color.Bold).SprintFunc()
	sectionStyle := color.New(color.FgCyan, color.Bold).SprintFunc()

	fmt.Println(headerStyle("Usage Statistics"))
	fmt.Printf("Total conversations: %d\n", sc.totalConversations)
	if sc.totalTokens > 0 {
		fmt.Printf("Total tokens: %s\n", formatTokenCount(sc.totalTokens))
	}
	fmt.Println()

	sc.printDailyStats(sectionStyle, showAll)
	sc.printHourlyStats(sectionStyle, showAll)
	sc.printAgentStats(sectionStyle, showAll)
	sc.printModelStats(sectionStyle, showAll)
}

// printDailyStats prints daily usage statistics
func (sc *StatsCollector) printDailyStats(sectionStyle func(a ...interface{}) string, showAll bool) {
	fmt.Println(sectionStyle("Daily Usage:"))

	type dailyUsage struct {
		date  string
		count int
	}

	var sortedDays []dailyUsage
	for date, stats := range sc.dayStats {
		sortedDays = append(sortedDays, dailyUsage{date: date, count: stats.Count})
	}

	sort.Slice(sortedDays, func(i, j int) bool {
		return sortedDays[i].date > sortedDays[j].date
	})

	// Show last 7 days
	lastDays := sortedDays
	if len(lastDays) > 7 && !showAll {
		lastDays = lastDays[:7]
	}

	for _, u := range lastDays {
		if tokens := sc.dayStats[u.date].Tokens; tokens > 0 {
			fmt.Printf("  %s: %d conversations (%s tokens)\n", u.date, u.count, formatTokenCount(tokens))
		} else {
			fmt.Printf("  %s: %d conversations\n", u.date, u.count)
		}
	}
	fmt.Println()
}

// printHourlyStats prints hourly usage statistics
func (sc *StatsCollector) printHourlyStats(sectionStyle func(a ...interface{}) string, showAll bool) {
	fmt.Println(sectionStyle("Hourly Usage:"))

	type hourlyUsage struct {
		hour  int
		count int
	}

	var sortedHours []hourlyUsage
	for hour, stats := range sc.hourStats {
		sortedHours = append(sortedHours, hourlyUsage{hour: hour, count: stats.Count})
	}

	sort.Slice(sortedHours, func(i, j int) bool {
		return sortedHours[i].count > sortedHours[j].count
	})

	// Show top 5 hours
	topHours := sortedHours
	if len(topHours) > 5 && !showAll {
		topHours = topHours[:5]
	}

	for _, usage := range topHours {
		fmt.Printf("  %02d:00-%02d:59: %d conversations\n", usage.hour, usage.hour, usage.count)
	}
	fmt.Println()
}

// printAgentStats prints agent usage statistics
func (sc *StatsCollector) printAgentStats(sectionStyle func(a ...interface{}) string, showAll bool) {
	fmt.Println(sectionStyle("Agent Usage:"))

	type agentUsage struct {
		name  string
		count int
	}

	var sortedAgents []agentUsage
	for name, stats := range sc.agentStats {
		sortedAgents = append(sortedAgents, agentUsage{name: name, count: stats.Count})
	}

	sort.Slice(sortedAgents, func(i, j int) bool {
		return sortedAgents[i].count > sortedAgents[j].count
	})

	// Show top 10 agents
	topAgents := sortedAgents
	if len(topAgents) > 10 && !showAll {
		topAgents = topAgents[:10]
	}

	for _, u := range topAgents {
		if tokens := sc.agentStats[u.name].Tokens; tokens > 0 {
			fmt.Printf("  +%s: %d conversations (%s tokens)\n", u.name, u.count, formatTokenCount(tokens))
		} else {
			fmt.Printf("  +%s: %d conversations\n", u.name, u.count)
		}
	}
	fmt.Println()
}

// printModelStats prints model usage statistics
func (sc *StatsCollector) printModelStats(sectionStyle func(a ...interface{}) string, showAll bool) {
	fmt.Println(sectionStyle("Model Usage:"))

	type modelUsage struct {
		name  string
		count int
	}

	var sortedModels []modelUsage
	for name, stats := range sc.modelStats {
		sortedModels = append(sortedModels, modelUsage{name: name, count: stats.Count})
	}

	sort.Slice(sortedModels, func(i, j int) bool {
		return sortedModels[i].count > sortedModels[j].count
	})

	// Show top 10 models
	topModels := sortedModels
	if len(topModels) > 10 && !showAll {
		topModels = topModels[:10]
	}

	for _, u := range topModels {
		if tokens := sc.modelStats[u.name].Tokens; tokens > 0 {
			fmt.Printf("  %s: %d conversations (%s tokens)\n", u.name, u.count, formatTokenCount(tokens))
		} else {
			fmt.Printf("  %s: %d conversations\n", u.name, u.count)
		}
	}
}

// formatTokenCount renders a token count with comma separators for readability.
// e.g. 45230 → "45,230"
func formatTokenCount(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	// Insert commas from the right
	var result strings.Builder
	for i, ch := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result.WriteByte(',')
		}
		result.WriteRune(ch)
	}
	return result.String()
}
