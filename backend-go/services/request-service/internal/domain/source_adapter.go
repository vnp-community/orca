package domain

import (
	"context"
	"errors"
)

type SourceQuery struct {
	Text  string
	Kinds []SourceKind
	Limit int
}

type SourceFilter struct {
	Kinds []SourceKind
}

type ContextSourceRef struct {
	SourceID string
	Ref      string
	Title    string
}

type ChangeFilter struct {
	Kinds []SourceKind
}

type SourceChange struct {
	SourceID string
	Ref      string
	Event    string // "created", "updated", "deleted"
}

type SourceAdapter interface {
	Key() string
	Search(ctx context.Context, q SourceQuery) ([]SourceItem, error)
	Get(ctx context.Context, ref string) (SourceItem, error)
	List(ctx context.Context, f SourceFilter) ([]ContextSourceRef, error)
	Subscribe(ctx context.Context, f ChangeFilter) (<-chan SourceChange, error)
}

type SourceItem struct {
	SourceID    string
	Ref         string
	Title       string
	Content     string
	RetrievedAt string
	Freshness   string // "fresh", "stale", "unknown"
	Trust       Trust
	Digest      string
	Size        int
}

var ErrSubscribeUnsupported = errors.New("subscribe unsupported")

func (i SourceItem) Validate() error {
	if i.SourceID == "" {
		return errors.New("missing SourceID")
	}
	if i.Ref == "" {
		return errors.New("missing Ref")
	}
	if i.RetrievedAt == "" {
		return errors.New("missing RetrievedAt")
	}
	if i.Freshness != "fresh" && i.Freshness != "stale" && i.Freshness != "unknown" {
		return errors.New("invalid Freshness")
	}
	if i.Trust != TrustHigh && i.Trust != TrustMedium && i.Trust != TrustLow {
		return errors.New("invalid Trust")
	}
	if len(i.Digest) != 64 {
		return errors.New("invalid Digest")
	}
	if i.Size < 0 {
		return errors.New("invalid Size")
	}
	return nil
}
