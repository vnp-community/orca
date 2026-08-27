// Package domain — file I/O value objects, parallel to this package's
// existing git types (GitStatus, DiffResult, etc.). See TASK-049/TASK-050.
package domain

// DirEntry is one entry returned by FilesystemExecutor.ReadDir.
type DirEntry struct {
	Name        string
	IsDirectory bool
}

// FileStat is FilesystemExecutor.Stat's result.
type FileStat struct {
	Exists           bool
	IsDirectory      bool
	SizeBytes        int64
	ModifiedAtUnixMs int64
}

// SearchOptions parameterizes FilesystemExecutor.Search.
type SearchOptions struct {
	Pattern    string
	IsRegex    bool
	PathGlob   string
	MaxResults int
}

// SearchMatch is one result line from FilesystemExecutor.Search.
type SearchMatch struct {
	Path     string
	Line     int
	LineText string
}
