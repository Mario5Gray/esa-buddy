package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func createNewHistoryFile(cacheDir string, agentName string, conversation string) string {
	if agentName == "" {
		agentName = "default"
	}
	timestamp := time.Now().Format(historyTimeFormat)

	if _, ok := getConversationIndex(conversation); ok {
		return filepath.Join(cacheDir, fmt.Sprintf("---%s-%s.json", agentName, timestamp))
	}

	return filepath.Join(cacheDir, fmt.Sprintf("%s---%s-%s.json", conversation, agentName, timestamp))
}

func getConversationIndex(conversation string) (int, bool) {
	val, err := strconv.Atoi(conversation)
	if err != nil || val < 0 {
		return 0, false
	}

	return val - 1, true
}

func findHistoryFile(cacheDir string, conversation string) (string, error) {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return "", err
	}

	type fileEntry struct {
		name    string
		modTime time.Time
	}

	index, isIndex := getConversationIndex(conversation)

	var files []fileEntry

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			files = append(files, fileEntry{
				name:    entry.Name(),
				modTime: info.ModTime(),
			})
		}
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no history files found")
	}

	if isIndex {
		sort.Slice(files, func(i, j int) bool {
			return files[i].modTime.After(files[j].modTime)
		})

		if index < 0 || index >= len(files) {
			return "", fmt.Errorf("history file index %d out of range (0-%d)", index, len(files)-1)
		}

		return filepath.Join(cacheDir, files[index].name), nil
	}

	// Read the conversation ID from the json file and return file with that id
	for _, file := range files {
		if strings.HasPrefix(file.name, conversation+"---") {
			return filepath.Join(cacheDir, file.name), nil
		}
	}

	return "", fmt.Errorf("no history file found for conversation %s", conversation)
}

func getHistoryFilePath(cacheDir string, opts *CLIOptions) (string, bool) {
	if !opts.ContinueChat && !opts.RetryChat {
		cacheDir = setupCacheDirWithFallback()
		return createNewHistoryFile(cacheDir, opts.AgentName, opts.Conversation), false
	}

	if filePath, err := findHistoryFile(cacheDir, opts.Conversation); err == nil {
		return filePath, true
	}

	cacheDir = setupCacheDirWithFallback()
	return createNewHistoryFile(cacheDir, opts.AgentName, opts.Conversation), false
}

// getSortedHistoryFiles retrieves and sorts history files by modification time.
func getSortedHistoryFiles() ([]string, map[string]os.FileInfo, error) {
	cacheDir, err := setupCacheDir()
	if err != nil {
		return nil, nil, err
	}

	// Check if the directory exists
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		return nil, nil, wrapCacheError("access", cacheDir, fmt.Errorf("directory does not exist"))
	}

	// Read all .json files in the directory
	files, err := os.ReadDir(cacheDir)
	if err != nil {
		return nil, nil, wrapCacheError("read", cacheDir, err)
	}

	historyItems := make(map[string]os.FileInfo) // Store file info to sort by mod time later

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			info, err := file.Info()
			if err != nil {
				continue // Skip files we can't get info for
			}
			historyItems[file.Name()] = info
		}
	}

	if len(historyItems) == 0 {
		return nil, nil, wrapCacheError("find history files", cacheDir, fmt.Errorf("no history files found"))
	}

	// Sort files by modification time (most recent first)
	sortedFiles := make([]string, 0, len(historyItems))
	for name := range historyItems {
		sortedFiles = append(sortedFiles, name)
	}
	// Custom sort function
	sort.Slice(sortedFiles, func(i, j int) bool {
		return historyItems[sortedFiles[i]].ModTime().After(historyItems[sortedFiles[j]].ModTime())
	})

	return sortedFiles, historyItems, nil
}
