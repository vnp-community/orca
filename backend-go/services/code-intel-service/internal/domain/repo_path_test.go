package domain

import (
	"strings"
	"testing"
)

func TestNormalizeRepoPath(t *testing.T) {
	tests := []struct {
		name          string
		raw           string
		wsRoot        string
		plat          Platform
		expected      string
		expectErrCode string
	}{
		{
			name:     "windows_absolute_under_root",
			raw:      `C:\repo\src\a.ts`,
			wsRoot:   `c:\repo`,
			plat:     PlatformWin32,
			expected: "src/a.ts",
		},
		{
			name:          "traversal_outside_root",
			raw:           "/home/u/repo/../x",
			wsRoot:        "/home/u/repo",
			plat:          PlatformLinux,
			expectErrCode: "CODEINTEL_PATH_NOT_ALLOWED",
		},
		{
			name:          "wsl_unc_rejected",
			raw:           `\\wsl$\Ubuntu\home\u\a.ts`,
			wsRoot:        `/home/u/repo`,
			plat:          PlatformWin32,
			expectErrCode: "CODEINTEL_PATH_NOT_ALLOWED",
		},
		{
			name:     "linux_relative_backslashes_preserved",
			raw:      `a\b.ts`,
			wsRoot:   "",
			plat:     PlatformLinux,
			expected: `a\b.ts`,
		},
		{
			name:     "darwin_case_folded_prefix_preserves_case",
			raw:      "/Users/Alice/Project/Src/Component.tsx",
			wsRoot:   "/users/alice/project",
			plat:     PlatformDarwin,
			expected: "Src/Component.tsx",
		},
		{
			name:     "unicode_nfd_to_nfc",
			raw:      "ti\u0301nh.ts",
			wsRoot:   "",
			plat:     PlatformLinux,
			expected: "t\u00ednh.ts",
		},
		{
			name:     "clean_dot_and_double_slashes",
			raw:      "./src//a.ts",
			wsRoot:   "",
			plat:     PlatformLinux,
			expected: "src/a.ts",
		},
		{
			name:          "over_max_length_no_panic",
			raw:           strings.Repeat("a/", 2500) + "file.ts",
			wsRoot:        "",
			plat:          PlatformLinux,
			expectErrCode: "CODEINTEL_PATH_NOT_ALLOWED",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeRepoPath(tc.raw, tc.wsRoot, tc.plat)
			if tc.expectErrCode != "" {
				if err == nil {
					t.Fatalf("expected error containing %s, got nil", tc.expectErrCode)
				}
				if !strings.Contains(err.Error(), tc.expectErrCode) {
					t.Fatalf("expected error code %s, got %v", tc.expectErrCode, err)
				}
				// Verify error message does not leak raw input
				if len(tc.raw) > 5 && strings.Contains(err.Error(), tc.raw) {
					t.Fatalf("error message leaks raw input: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestFoldCaseForCompare(t *testing.T) {
	if FoldCaseForCompare("Src/File.go", PlatformDarwin) != "src/file.go" {
		t.Fatal("expected darwin to fold case")
	}
	if FoldCaseForCompare("Src/File.go", PlatformWin32) != "src/file.go" {
		t.Fatal("expected win32 to fold case")
	}
	if FoldCaseForCompare("Src/File.go", PlatformLinux) != "Src/File.go" {
		t.Fatal("expected linux to preserve case")
	}
}
