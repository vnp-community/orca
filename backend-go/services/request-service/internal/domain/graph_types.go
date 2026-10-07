package domain

type SymbolVector struct {
	RefKey string
	Path   string
}

func NormalizeSymbolRefKey(key string) string {
	return key
}

func NormalizeRepoPath(path string) string {
	return path
}
