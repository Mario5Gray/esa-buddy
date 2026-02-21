package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/sashabaranov/go-openai"
)

// listHistory lists available history files in the cache directory
func listHistory(showAll bool) {
	sortedFiles, _, err := getSortedHistoryFiles() // Use blank identifier for unused historyItems
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
		conversation := ""
		agentName := "unknown"
		timestampStr := "unknown"
		if len(parts) == 5 {
			conversation = parts[0]
			agentName = parts[3]
			timestampStr = parts[4]
			if parsedTime, err := time.Parse("20060102-150405", timestampStr); err == nil {
				timestampStr = parsedTime.Format("2006-01-02 15:04:05")
			}
		}

		// Get first user query
		cacheDir, _ := setupCacheDir()
		historyFilePath := filepath.Join(cacheDir, fileName)
		var query string
		if historyData, err := os.ReadFile(historyFilePath); err == nil {
			var history ConversationHistory
			if err := json.Unmarshal(historyData, &history); err == nil {
				prevMessage := ""
				for _, msg := range history.Messages {
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

		if len(conversation) > 0 {
			conversation = fmt.Sprintf("(%s) ", conversation)
		}

		fmt.Printf(" %2d: %s%s %s %s\n",
			i+1,
			conversation,
			highPriStyle("+"+agentName),
			query,
			lowPriStyle(timestampStr),
		)
	}
}

// handleShowHistory displays the content of a specific history file in the specified format.
func handleShowHistory(conversation string, outputFormat string) {
	historyFilePath, history, ok := readHistoryFile(conversation)
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

func readHistoryFile(conversation string) (string, ConversationHistory, bool) {
	cacheDir, err := setupCacheDir()
	if err != nil {
		if strings.Contains(err.Error(), "no history files found") || strings.Contains(err.Error(), "cache directory does not exist") {
			color.Yellow(err.Error())
		} else {
			printError(err.Error())
		}
		return "", ConversationHistory{}, false
	}

	historyFilePath, err := findHistoryFile(cacheDir, conversation)
	if err != nil {
		printError(fmt.Sprintf("Error finding history file for %s", conversation))
		return "", ConversationHistory{}, false
	}

	historyData, err := os.ReadFile(historyFilePath)
	if err != nil {
		printError(fmt.Sprintf("Error reading history file for %s", conversation))
		return "", ConversationHistory{}, false
	}

	var history ConversationHistory
	err = json.Unmarshal(historyData, &history)
	if err != nil {
		printError(fmt.Sprintf("Error loading history file for %s", conversation))
		return "", ConversationHistory{}, false
	}

	return historyFilePath, history, true
}

// handleShowOutput displays output from a specific history file.
func handleShowOutput(conversation string, pretty bool) {
	_, history, ok := readHistoryFile(conversation)
	if !ok {
		return
	}

	printOutput(history, pretty)
}

// handleShowStats analyzes history files and displays usage statistics
func handleShowStats(showAll bool) {
	// Get all history files
	sortedFiles, fileInfo, err := getSortedHistoryFiles()
	if err != nil {
		if strings.Contains(err.Error(), "no history files found") || strings.Contains(err.Error(), "cache directory does not exist") {
			color.Yellow(err.Error())
		} else {
			color.Red(err.Error())
		}
		return
	}

	cacheDir, _ := setupCacheDir()
	collector := NewStatsCollector()

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
