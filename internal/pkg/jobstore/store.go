// Package jobstore provides file storage for background job outputs (e.g. export files).
package jobstore

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// FileStore abstracts write/read operations for job output files.
type FileStore interface {
	Write(jobID, filename string, r io.Reader) (storagePath string, err error)
	Read(storagePath string) (io.ReadCloser, error)
}

// LocalFileStore stores files on the local filesystem under a configured base directory.
type LocalFileStore struct {
	basePath string
}

// NewLocalFileStore creates a LocalFileStore rooted at basePath.
// basePath is created if it does not exist.
func NewLocalFileStore(basePath string) (*LocalFileStore, error) {
	if err := os.MkdirAll(basePath, 0o750); err != nil {
		return nil, fmt.Errorf("jobstore: failed to create base dir %s: %w", basePath, err)
	}
	return &LocalFileStore{basePath: basePath}, nil
}

// Write saves the data from r to {basePath}/{jobID}_{filename} and returns the storage path.
func (s *LocalFileStore) Write(jobID, filename string, r io.Reader) (string, error) {
	dest := filepath.Join(s.basePath, jobID+"_"+filename)
	f, err := os.Create(dest)
	if err != nil {
		return "", fmt.Errorf("jobstore: create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return "", fmt.Errorf("jobstore: write file: %w", err)
	}
	return dest, nil
}

// Read opens a previously stored file for reading.
func (s *LocalFileStore) Read(storagePath string) (io.ReadCloser, error) {
	f, err := os.Open(storagePath)
	if err != nil {
		return nil, fmt.Errorf("jobstore: open file: %w", err)
	}
	return f, nil
}
