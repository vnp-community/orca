package usecase

import (
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
)

// normalizeSparsePresetName ports backend/src/main/runtime/rpc/methods/
// sparse-presets.ts's normalizeSparsePresetName (legacy TS reference) 1:1.
func normalizeSparsePresetName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", apperrors.New(apperrors.KindInvalidArgument, "PROJECT_SPARSE_PRESET_NAME_REQUIRED", "preset name is required", nil)
	}
	if len(trimmed) > 80 {
		return "", apperrors.New(apperrors.KindInvalidArgument, "PROJECT_SPARSE_PRESET_NAME_TOO_LONG", "preset name is too long", nil)
	}
	return trimmed, nil
}

// normalizeSparsePresetDirectories ports backend/src/main/ipc/
// sparse-checkout-directories.ts's normalizeSparseDirectories (legacy TS
// reference) 1:1: trims each entry, rejects absolute paths, strips leading/
// trailing slashes, drops empty/"." entries, rejects any ".." path
// segment (traversal), and de-duplicates while preserving order.
func normalizeSparsePresetDirectories(directories []string) ([]string, error) {
	seen := make(map[string]bool, len(directories))
	out := make([]string, 0, len(directories))
	for _, raw := range directories {
		entry := strings.TrimSpace(raw)
		if isAbsoluteSparsePresetPath(entry) {
			return nil, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_SPARSE_PRESET_DIRECTORY_INVALID", "preset directories must be repo-relative paths", nil)
		}
		entry = strings.ReplaceAll(entry, "\\", "/")
		entry = strings.Trim(entry, "/")
		if entry == "" || entry == "." {
			continue
		}
		for _, segment := range strings.Split(entry, "/") {
			if segment == ".." {
				return nil, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_SPARSE_PRESET_DIRECTORY_INVALID", "preset directories must be repo-relative paths", nil)
			}
		}
		if seen[entry] {
			continue
		}
		seen[entry] = true
		out = append(out, entry)
	}
	if len(out) == 0 {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_SPARSE_PRESET_DIRECTORIES_REQUIRED", "preset must have at least one directory", nil)
	}
	return out, nil
}

// isAbsoluteSparsePresetPath mirrors the legacy reference's
// isAbsoluteSparseDirectoryPath exactly: a leading "/" or "\", or a Windows
// drive-letter prefix (^[A-Za-z]:) counts as absolute.
func isAbsoluteSparsePresetPath(path string) bool {
	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "\\") {
		return true
	}
	if len(path) >= 2 && isASCIILetter(path[0]) && path[1] == ':' {
		return true // e.g. "C:\..." or "C:/..."
	}
	return false
}

func isASCIILetter(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}
