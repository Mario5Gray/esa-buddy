package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/meain/esa/internal/conversation"
	hist "github.com/meain/esa/internal/conversation/history"
	"github.com/meain/esa/internal/inspect"
	"github.com/meain/esa/internal/utils"
	"github.com/sashabaranov/go-openai"
)

// listHistory lists available history files in the cache directory
func listHistory(showAll bool) {
	sortedFiles, _, err := utils.GetSortedHistoryFiles() // Use blank identifier for unused historyItems
	if err != nil {
		// Handle specific errors or just print the message
		if strings.Contains(err.Error(), "no history files found") || strings.Contains(err.Error(), "cache directory does not exist") {
			printWarning(err.Error())
		} else {
			printError(err.Error())
		}
		return
	}

	highPriStyle := color.New(color.FgHiCyan, color.Bold).SprintFunc()
	lowPriStyle := color.New(color.FgHiWhite, color.Italic).SprintFunc()

	fmt.Printf("Available conversation histories (total: %d):\n", len(sortedFiles))

	// Determine how many items to show
	itemsToShow := sortedFiles
	if !showAll {
		if len(sortedFiles) > 15 {
			itemsToShow = sortedFiles[:15]
		}
	}

	for i, fileName := range itemsToShow {
		parts := strings.SplitN(strings.TrimSuffix(fileName, ".json"), "-", 5)
		conversationID := ""
		agentName := "unknown"
		timestampStr := "unknown"
		if len(parts) == 5 {
			conversationID = parts[0]
			agentName = parts[3]
			timestampStr = parts[4]
			if parsedTime, err := time.Parse("20060102-150405", timestampStr); err == nil {
				timestampStr = parsedTime.Format("2006-01-02 15:04:05")
			}
		}

		// Get first user query
		cacheDir, _ := utils.SetupCacheDir()
		historyFilePath := filepath.Join(cacheDir, fileName)
		var query string
		if historyData, err := os.ReadFile(historyFilePath); err == nil {
			var historyDataObj hist.ConversationHistory
			if err := json.Unmarshal(historyData, &historyDataObj); err == nil {
				prevMessage := ""
				for _, msg := range historyDataObj.Messages {
					if msg.Role == openai.ChatMessageRoleAssistant {
						query = strings.ReplaceAll(prevMessage, "\n", " ")
						if len(query) > 60 {
							query = query[:57] + "..."
						}
						break
					}

					prevMessage = msg.Content
				}
			}
		}

		if len(conversationID) > 0 {
			conversationID = fmt.Sprintf("(%s) ", conversationID)
		}

		fmt.Printf(" %2d: %s%s %s %s\n",
			i+1,
			conversationID,
			highPriStyle("+"+agentName),
			query,
			lowPriStyle(timestampStr),
		)
	}
}

// handleShowHistory displays the content of a specific history file in the specified format.
func handleShowHistory(conversationID string, outputFormat string) {
	historyFilePath, history, ok := readHistoryFile(conversationID)
	if !ok {
		return
	}

	switch outputFormat {
	case "json":
		printHistoryJSON(history)
	case "markdown":
		printHistoryMarkdown(historyFilePath, history)
	default: // "text"
		printHistoryText(historyFilePath, history)
	}
}

func readHistoryFile(conversationID string) (string, hist.ConversationHistory, bool) {
	cacheDir, err := utils.SetupCacheDir()
	if err != nil {
		if strings.Contains(err.Error(), "no history files found") || strings.Contains(err.Error(), "cache directory does not exist") {
			color.Yellow(err.Error())
		} else {
			printError(err.Error())
		}
		return "", hist.ConversationHistory{}, false
	}

	historyFilePath, err := utils.FindHistoryFile(cacheDir, conversationID)
	if err != nil {
		printError(fmt.Sprintf("Error finding history file for %s", conversationID))
		return "", hist.ConversationHistory{}, false
	}

	historyData, err := os.ReadFile(historyFilePath)
	if err != nil {
		printError(fmt.Sprintf("Error reading history file for %s", conversationID))
		return "", hist.ConversationHistory{}, false
	}

	var historyDataObj hist.ConversationHistory
	err = json.Unmarshal(historyData, &historyDataObj)
	if err != nil {
		printError(fmt.Sprintf("Error loading history file for %s", conversationID))
		return "", hist.ConversationHistory{}, false
	}

	return historyFilePath, historyDataObj, true
}

// handleShowOutput displays output from a specific history file.
func handleShowOutput(conversationID string, pretty bool) {
	_, history, ok := readHistoryFile(conversationID)
	if !ok {
		return
	}

	printOutput(history, pretty)
}

func handleInspect(conversationID string, format string) {
	historyFilePath, history, ok := readHistoryFile(conversationID)
	if !ok {
		return
	}

	tape := inspect.BuildTape(historyFilePath, history)
	var renderer inspect.Renderer = inspect.TextRenderer{}
	if format == "json" {
		renderer = inspect.JSONRenderer{}
	}
	out, err := renderer.Render(tape)
	if err != nil {
		printError(fmt.Sprintf("Error rendering inspection: %v", err))
		return
	}
	fmt.Println(out)
}

// handleShowStats analyzes history files and displays usage statistics
func handleShowStats(showAll bool) {
	// Get all history files
	sortedFiles, fileInfo, err := utils.GetSortedHistoryFiles()
	if err != nil {
		if strings.Contains(err.Error(), "no history files found") || strings.Contains(err.Error(), "cache directory does not exist") {
			color.Yellow(err.Error())
		} else {
			color.Red(err.Error())
		}
		return
	}

	cacheDir, _ := utils.SetupCacheDir()
	collector := conversation.NewStatsCollector()

	// Process each history file
	for _, fileName := range sortedFiles {
		historyFilePath := filepath.Join(cacheDir, fileName)
		fileModTime := fileInfo[fileName].ModTime()

		if err := collector.ProcessHistoryFile(historyFilePath, fileName, fileModTime); err != nil {
			color.Red("Error processing history file %s: %v", fileName, err)
		}
	}

	collector.PrintStatistics(showAll)
}
