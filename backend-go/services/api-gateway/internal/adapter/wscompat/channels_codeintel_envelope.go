package wscompat

import (
	"encoding/json"
	"time"

	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type wireSourceInfo struct {
	Tool      string  `json:"tool"`
	Version   string  `json:"version"`
	IndexedAt *string `json:"indexedAt"`
	Commit    *string `json:"commit"`
	LineBase  *int    `json:"lineBase,omitempty"`
}

type wireEnvelope struct {
	Repo          string           `json:"repo,omitempty"`
	WorktreeID    string           `json:"worktreeId"`
	View          string           `json:"view"`
	Sources       []wireSourceInfo `json:"sources"`
	HeadCommit    *string          `json:"headCommit"`
	Stale         bool             `json:"stale"`
	Truncated     bool             `json:"truncated"`
	TotalCount    int64            `json:"totalCount"`
	Etag          string           `json:"etag"`
	FromCache     bool             `json:"fromCache"`
	GeneratedAt   string           `json:"generatedAt"`
	NotModified   bool             `json:"notModified,omitempty"`
	NextPageToken string           `json:"nextPageToken,omitempty"`
	Data          json.RawMessage  `json:"data,omitempty"`
}

func encodeEnvelope(meta *codeintelv1.ResultMeta, data proto.Message, nextPageToken string) (json.RawMessage, error) {
	if meta == nil {
		meta = &codeintelv1.ResultMeta{}
	}

	viewName := "unknown"
	if meta.View != 0 {
		viewFd := meta.ProtoReflect().Descriptor().Fields().ByName("view")
		if viewFd != nil {
			viewName = enumWire(viewFd.Enum(), protoreflect.EnumNumber(meta.View))
		}
	}

	sources := make([]wireSourceInfo, 0, len(meta.Sources))
	for _, s := range meta.Sources {
		toolName := "unknown"
		if s.Tool != 0 {
			toolFd := s.ProtoReflect().Descriptor().Fields().ByName("tool")
			if toolFd != nil {
				toolName = enumWire(toolFd.Enum(), protoreflect.EnumNumber(s.Tool))
			}
		}

		var indexedAt *string
		if s.IndexedAt != "" {
			idx := s.IndexedAt
			indexedAt = &idx
		}

		var commit *string
		if s.Commit != "" {
			c := s.Commit
			commit = &c
		}

		var lineBase *int
		if s.LineBase > 0 {
			lb := int(s.LineBase)
			lineBase = &lb
		}

		sources = append(sources, wireSourceInfo{
			Tool:      toolName,
			Version:   s.Version,
			IndexedAt: indexedAt,
			Commit:    commit,
			LineBase:  lineBase,
		})
	}

	var headCommit *string
	if meta.HeadCommit != "" {
		hc := meta.HeadCommit
		headCommit = &hc
	}

	generatedAt := ""
	if meta.GeneratedAt != nil {
		t := meta.GeneratedAt.AsTime().UTC()
		if meta.GeneratedAt.Nanos == 0 {
			generatedAt = t.Format(time.RFC3339)
		} else {
			generatedAt = t.Format(time.RFC3339Nano)
		}
	} else {
		generatedAt = time.Now().UTC().Format(time.RFC3339)
	}

	env := wireEnvelope{
		Repo:          meta.Repo,
		WorktreeID:    meta.WorktreeId,
		View:          viewName,
		Sources:       sources,
		HeadCommit:    headCommit,
		Stale:         meta.Stale,
		Truncated:     meta.Truncated,
		TotalCount:    meta.TotalCount,
		Etag:          meta.Etag,
		FromCache:     meta.FromCache,
		GeneratedAt:   generatedAt,
		NextPageToken: nextPageToken,
	}

	if meta.NotModified {
		env.NotModified = true
	} else if data != nil {
		encodedData, err := encodeCodeIntelWire(data)
		if err != nil {
			return nil, err
		}
		env.Data = encodedData
	}

	b, err := json.Marshal(env)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}
