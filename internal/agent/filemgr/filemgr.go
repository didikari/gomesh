package filemgr

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"gomesh/internal/protocol"
)

// List returns the contents of a directory
func List(targetPath string) (string, []protocol.FileEntry, error) {
	if targetPath == "" || targetPath == "." {
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			targetPath = home
		} else {
			targetPath, _ = os.Getwd()
		}
	}

	targetPath = filepath.Clean(targetPath)
	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return targetPath, nil, fmt.Errorf("invalid path: %w", err)
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return absPath, nil, fmt.Errorf("failed reading directory: %w", err)
	}

	var results []protocol.FileEntry
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}

		itemPath := filepath.ToSlash(filepath.Join(absPath, e.Name()))
		results = append(results, protocol.FileEntry{
			Name:    e.Name(),
			Path:    itemPath,
			Size:    info.Size(),
			Mode:    info.Mode().String(),
			ModTime: info.ModTime().UnixMilli(),
			IsDir:   e.IsDir(),
		})
	}

	// Sort directories first, then alphabetical by name
	sort.Slice(results, func(i, j int) bool {
		if results[i].IsDir != results[j].IsDir {
			return results[i].IsDir
		}
		return results[i].Name < results[j].Name
	})

	return filepath.ToSlash(absPath), results, nil
}

// ReadChunk reads a chunk of a file at a specific offset
func ReadChunk(filePath string, offset int64, chunkSize int) (string, bool, error) {
	if chunkSize <= 0 || chunkSize > 512*1024 {
		chunkSize = 64 * 1024 // 64KB default
	}

	f, err := os.Open(filePath)
	if err != nil {
		return "", false, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return "", false, fmt.Errorf("failed to seek: %w", err)
	}

	buf := make([]byte, chunkSize)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return "", false, fmt.Errorf("failed reading chunk: %w", err)
	}

	isEOF := err == io.EOF || n < chunkSize
	encoded := base64.StdEncoding.EncodeToString(buf[:n])
	return encoded, isEOF, nil
}

// WriteChunk writes data chunk to target file
func WriteChunk(filePath string, offset int64, base64Data string, isLast bool) error {
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return fmt.Errorf("invalid base64 payload: %w", err)
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed creating parent directory: %w", err)
	}

	var flag int
	if offset == 0 {
		flag = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	} else {
		flag = os.O_WRONLY
	}

	f, err := os.OpenFile(filePath, flag, 0644)
	if err != nil {
		return fmt.Errorf("failed opening file for writing: %w", err)
	}
	defer f.Close()

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek: %w", err)
	}

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("failed writing bytes: %w", err)
	}

	return nil
}

// Action executes filesystem operations (delete, mkdir)
func Action(action, targetPath string) error {
	switch action {
	case "delete":
		return os.RemoveAll(targetPath)
	case "mkdir":
		return os.MkdirAll(targetPath, 0755)
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}
