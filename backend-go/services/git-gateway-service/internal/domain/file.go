// Package domain — file I/O value objects, parallel to this package's
// existing git types (GitStatus, DiffResult, etc.). See TASK-049/TASK-050.
package domain

// DirEntry is one entry returned by FilesystemExecutor.ReadDir.
type DirEntry struct {
	Name        string `json:"name"`
	IsDirectory bool   `json:"isDirectory"`
	// SizeBytes is 0 for directories. Populated from the same source
	// FileStat.SizeBytes already uses (os.Stat locally, the agent's
	// fs.stat/fs.readDir response over relay) — added SOL-PW-02.
	SizeBytes int64 `json:"sizeBytes"`
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
