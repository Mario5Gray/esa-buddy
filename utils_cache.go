package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// CacheError represents errors related to cache directory operations
type CacheError struct {
	Operation string
	Path      string
	Err       error
}

func (e *CacheError) Error() string {
	return fmt.Sprintf("cache %s failed for path '%s': %v", e.Operation, e.Path, e.Err)
}

// FileError represents errors related to file operations
type FileError struct {
	Operation string
	Path      string
	Err       error
}

func (e *FileError) Error() string {
	return fmt.Sprintf("file %s failed for path '%s': %v", e.Operation, e.Path, e.Err)
}

// wrapCacheError wraps an error with cache context
func wrapCacheError(operation, path string, err error) error {
	if err == nil {
		return nil
	}
	return &CacheError{Operation: operation, Path: path, Err: err}
}

// wrapFileError wraps an error with file context
func wrapFileError(operation, path string, err error) error {
	if err == nil {
		return nil
	}
	return &FileError{Operation: operation, Path: path, Err: err}
}

// setupCacheDir ensures the cache directory exists and returns its path.
func setupCacheDir() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", wrapCacheError("get user cache directory", "", err)
	}
	esaDir := filepath.Join(cacheDir, "esa")
	if err := os.MkdirAll(esaDir, 0755); err != nil {
		return "", wrapCacheError("create directory", esaDir, err)
	}
	return esaDir, nil
}

// setupCacheDirWithFallback ensures the cache directory exists and handles errors gracefully
func setupCacheDirWithFallback() string {
	cacheDir, err := setupCacheDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not setup cache directory: %v\n", err)
		// Fallback to temp directory
		return filepath.Join(os.TempDir(), "esa")
	}
	return cacheDir
}
