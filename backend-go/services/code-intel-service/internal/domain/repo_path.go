package domain

import (
	"fmt"
	"path"
	"strings"

	"golang.org/x/text/unicode/norm"
)

type Platform string

const (
	PlatformLinux  Platform = "linux"
	PlatformDarwin Platform = "darwin"
	PlatformWin32  Platform = "win32"
)

const (
	maxPathLength = 4096
)

type PathNotAllowedError struct {
	Message string
}

func (e PathNotAllowedError) Error() string {
	return fmt.Sprintf("CODEINTEL_PATH_NOT_ALLOWED: %s", e.Message)
}

func pathNotAllowedErr(msg string) error {
	return PathNotAllowedError{Message: msg}
}

// NormalizeRepoPath normalizes a raw file path relative to workspaceRoot.
// It defends against traversal, UNC paths, and paths outside workspaceRoot.
func NormalizeRepoPath(raw, workspaceRoot string, plat Platform) (string, error) {
	if raw == "" || len(raw) >= maxPathLength || strings.ContainsRune(raw, '\x00') {
		return "", pathNotAllowedErr("path is invalid or exceeds maximum length")
	}

	// Reject UNC prefixes
	lowerRaw := strings.ToLower(raw)
	if strings.HasPrefix(lowerRaw, `\\wsl$\`) ||
		strings.HasPrefix(lowerRaw, `\\wsl.localhost\`) ||
		strings.HasPrefix(lowerRaw, `\\?\`) ||
		strings.HasPrefix(lowerRaw, `//wsl$/`) ||
		strings.HasPrefix(lowerRaw, `//wsl.localhost/`) {
		return "", pathNotAllowedErr("UNC paths are not allowed")
	}

	cleanRaw := raw
	cleanWs := workspaceRoot

	// Convert backslashes only on Windows
	if plat == PlatformWin32 {
		cleanRaw = strings.ReplaceAll(cleanRaw, `\`, `/`)
		cleanWs = strings.ReplaceAll(cleanWs, `\`, `/`)
	}

	// Normalize Unicode to NFC
	cleanRaw = norm.NFC.String(cleanRaw)
	cleanWs = norm.NFC.String(cleanWs)

	// Clean paths
	cleanRaw = path.Clean(cleanRaw)
	if cleanWs != "" {
		cleanWs = path.Clean(cleanWs)
	}

	// Check if raw is absolute
	isAbs := path.IsAbs(cleanRaw) || (plat == PlatformWin32 && len(cleanRaw) >= 2 && cleanRaw[1] == ':')

	if isAbs {
		if cleanWs == "" {
			return "", pathNotAllowedErr("absolute path without workspace root is not allowed")
		}

		compareRaw := cleanRaw
		compareWs := cleanWs
		if plat == PlatformWin32 || plat == PlatformDarwin {
			compareRaw = strings.ToLower(compareRaw)
			compareWs = strings.ToLower(compareWs)
		}

		if compareRaw == compareWs {
			return "", pathNotAllowedErr("workspace root itself cannot be a repo file path")
		}

		prefix := compareWs + "/"
		if !strings.HasPrefix(compareRaw, prefix) {
			return "", pathNotAllowedErr("path outside workspace root is not allowed")
		}

		// Cut workspace root prefix from cleanRaw
		cleanRaw = cleanRaw[len(cleanWs)+1:]
	}

	cleanRaw = path.Clean(cleanRaw)
	if cleanRaw == "." || cleanRaw == "/" {
		return "", pathNotAllowedErr("empty or root path is not allowed")
	}

	if cleanRaw == ".." || strings.HasPrefix(cleanRaw, "../") || strings.Contains(cleanRaw, "/../") {
		return "", pathNotAllowedErr("path traversal outside repo is not allowed")
	}

	cleanRaw = strings.TrimPrefix(cleanRaw, "./")
	cleanRaw = strings.TrimPrefix(cleanRaw, "/")

	return cleanRaw, nil
}

// FoldCaseForCompare folds case for filesystem comparisons on case-insensitive platforms.
func FoldCaseForCompare(p string, plat Platform) string {
	if plat == PlatformWin32 || plat == PlatformDarwin {
		return strings.ToLower(p)
	}
	return p
}
