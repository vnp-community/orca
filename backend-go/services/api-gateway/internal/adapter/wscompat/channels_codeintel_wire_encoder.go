package wscompat

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	maxSafeInt64 = int64(1<<53 - 1)
	minSafeInt64 = -maxSafeInt64
)

// codeIntelNeverOnWire lists fields that must NEVER appear on the client wire (UI-API 2.2).
var codeIntelNeverOnWire = map[string]bool{
	// UI-API 2.2: devServerId is an internal backend identifier
	"orca.codeintel.v1.ResultMeta.devServerId": true,
}

// codeIntelNullableFields lists fields that serialize as null when absent or zero (UI-API §4).
var codeIntelNullableFields = map[string]bool{
	// UI-API 2.2 / §4.1: ResultMeta / SourceInfo nullable fields
	"orca.codeintel.v1.ResultMeta.headCommit": true,
	"orca.codeintel.v1.SourceInfo.indexedAt":  true,
	"orca.codeintel.v1.SourceInfo.commit":     true,

	// UI-API 4.1: IndexStatus / Reindex nullable fields
	"orca.codeintel.v1.ToolIndexStatus.pendingChanges":  true,
	"orca.codeintel.v1.ReindexJob.percent":              true,
	"orca.codeintel.v1.ActiveReindexJob.percent":        true,

	// UI-API 4.2: Graph view nullable fields
	"orca.codeintel.v1.FlowSummary.entry":          true,
	"orca.codeintel.v1.FlowSummary.terminal":       true,
	"orca.codeintel.v1.RouteEdge.handler":          true,
	"orca.codeintel.v1.SymbolDetail.source":        true,
	"orca.codeintel.v1.SymbolDetail.sourceOmitted": true,
	"orca.codeintel.v1.AffectedCluster.id":         true,
}

func isNeverOnWire(md protoreflect.MessageDescriptor, fd protoreflect.FieldDescriptor) bool {
	full := string(md.FullName()) + "." + fd.JSONName()
	if codeIntelNeverOnWire[full] {
		return true
	}
	name := string(fd.Name())
	if strings.HasSuffix(name, "_absolute_path") || name == "absolute_path" || name == "workspace_root" || name == "dev_server_id" {
		return true
	}
	return false
}

func isZeroScalar(fd protoreflect.FieldDescriptor, val protoreflect.Value) bool {
	switch fd.Kind() {
	case protoreflect.StringKind:
		return val.String() == ""
	case protoreflect.Int32Kind, protoreflect.Int64Kind, protoreflect.Sint32Kind, protoreflect.Sint64Kind, protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind:
		return val.Int() == 0
	case protoreflect.Uint32Kind, protoreflect.Uint64Kind, protoreflect.Fixed32Kind, protoreflect.Fixed64Kind:
		return val.Uint() == 0
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return val.Float() == 0
	case protoreflect.BoolKind:
		return !val.Bool()
	default:
		return false
	}
}

func encodeValue(fd protoreflect.FieldDescriptor, val protoreflect.Value, buf *bytes.Buffer) error {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		if val.Bool() {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		buf.WriteString(strconv.FormatInt(val.Int(), 10))
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		buf.WriteString(strconv.FormatUint(val.Uint(), 10))
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		n := val.Int()
		if n > maxSafeInt64 || n < minSafeInt64 {
			return errors.New("CODEINTEL_RESULT_INVALID: integer exceeds safe JS range 2^53-1")
		}
		buf.WriteString(strconv.FormatInt(n, 10))
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		u := val.Uint()
		if u > uint64(maxSafeInt64) {
			return errors.New("CODEINTEL_RESULT_INVALID: integer exceeds safe JS range 2^53-1")
		}
		buf.WriteString(strconv.FormatUint(u, 10))
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		buf.WriteString(strconv.FormatFloat(val.Float(), 'f', -1, 64))
	case protoreflect.StringKind:
		b, _ := json.Marshal(val.String())
		buf.Write(b)
	case protoreflect.BytesKind:
		return errors.New("CODEINTEL_RESULT_INVALID: bytes fields not allowed on wire")
	case protoreflect.EnumKind:
		wire := enumWire(fd.Enum(), val.Enum())
		b, _ := json.Marshal(wire)
		buf.Write(b)
	case protoreflect.MessageKind, protoreflect.GroupKind:
		subMsg := val.Message()
		if subMsg.Descriptor().FullName() == "google.protobuf.Timestamp" {
			ts := subMsg.Interface().(*timestamppb.Timestamp)
			if ts == nil || !ts.IsValid() {
				buf.WriteString("null")
			} else {
				t := ts.AsTime().UTC()
				b, _ := json.Marshal(t.Format(time.RFC3339Nano))
				buf.Write(b)
			}
			return nil
		}
		if subMsg.Descriptor().FullName() == "google.protobuf.Duration" {
			return errors.New("CODEINTEL_RESULT_INVALID: duration fields not allowed on wire")
		}
		return encodeMessageReflect(subMsg, buf)
	default:
		return fmt.Errorf("CODEINTEL_RESULT_INVALID: unsupported kind %v", fd.Kind())
	}
	return nil
}

func encodeMessageReflect(refl protoreflect.Message, buf *bytes.Buffer) error {
	if !refl.IsValid() {
		buf.WriteString("null")
		return nil
	}
	buf.WriteByte('{')
	md := refl.Descriptor()
	fields := md.Fields()
	first := true

	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		if isNeverOnWire(md, fd) {
			continue
		}

		if oneof := fd.ContainingOneof(); oneof != nil && !oneof.IsSynthetic() {
			if refl.WhichOneof(oneof) != fd {
				continue
			}
		}

		fullKey := string(md.FullName()) + "." + fd.JSONName()
		isNullable := codeIntelNullableFields[fullKey]

		// List (repeated)
		if fd.IsList() {
			if !first {
				buf.WriteByte(',')
			}
			first = false
			buf.WriteByte('"')
			buf.WriteString(fd.JSONName())
			buf.WriteString("\":[")
			list := refl.Get(fd).List()
			for j := 0; j < list.Len(); j++ {
				if j > 0 {
					buf.WriteByte(',')
				}
				item := list.Get(j)
				if fd.Kind() == protoreflect.MessageKind {
					if err := encodeMessageReflect(item.Message(), buf); err != nil {
						return err
					}
				} else {
					if err := encodeValue(fd, item, buf); err != nil {
						return err
					}
				}
			}
			buf.WriteByte(']')
			continue
		}

		// Map
		if fd.IsMap() {
			if !first {
				buf.WriteByte(',')
			}
			first = false
			buf.WriteByte('"')
			buf.WriteString(fd.JSONName())
			buf.WriteString("\":{")
			mapVal := refl.Get(fd).Map()
			mapEntry := fd.Message()
			valFd := mapEntry.Fields().ByNumber(2)
			mapFirst := true
			var keys []protoreflect.MapKey
			mapVal.Range(func(k protoreflect.MapKey, _ protoreflect.Value) bool {
				keys = append(keys, k)
				return true
			})
			sort.Slice(keys, func(a, b int) bool {
				return keys[a].String() < keys[b].String()
			})
			for _, k := range keys {
				if !mapFirst {
					buf.WriteByte(',')
				}
				mapFirst = false
				v := mapVal.Get(k)
				b, _ := json.Marshal(k.String())
				buf.Write(b)
				buf.WriteByte(':')
				if valFd.Kind() == protoreflect.MessageKind {
					if err := encodeMessageReflect(v.Message(), buf); err != nil {
						return err
					}
				} else {
					if err := encodeValue(valFd, v, buf); err != nil {
						return err
					}
				}
			}
			buf.WriteByte('}')
			continue
		}

		// Scalar or Message presence handling
		hasField := refl.Has(fd)
		if !hasField {
			if isNullable {
				if !first {
					buf.WriteByte(',')
				}
				first = false
				buf.WriteByte('"')
				buf.WriteString(fd.JSONName())
				buf.WriteString("\":null")
				continue
			}
			// Proto3 scalar without presence is always output, even when zero
			if !fd.HasPresence() && fd.Kind() != protoreflect.MessageKind {
				if !first {
					buf.WriteByte(',')
				}
				first = false
				buf.WriteByte('"')
				buf.WriteString(fd.JSONName())
				buf.WriteString("\":")
				val := refl.Get(fd)
				if err := encodeValue(fd, val, buf); err != nil {
					return err
				}
				continue
			}
			// Optional/message absent without nullable rule: omitted from wire
			continue
		}

		val := refl.Get(fd)
		if isNullable && isZeroScalar(fd, val) {
			if !first {
				buf.WriteByte(',')
			}
			first = false
			buf.WriteByte('"')
			buf.WriteString(fd.JSONName())
			buf.WriteString("\":null")
			continue
		}

		if !first {
			buf.WriteByte(',')
		}
		first = false
		buf.WriteByte('"')
		buf.WriteString(fd.JSONName())
		buf.WriteString("\":")
		if err := encodeValue(fd, val, buf); err != nil {
			return err
		}
	}

	buf.WriteByte('}')
	return nil
}

// encodeCodeIntelWire encodes a protobuf message into camelCase JSON following the UI-API contract.
func encodeCodeIntelWire(m proto.Message) (json.RawMessage, error) {
	if m == nil {
		return json.RawMessage("null"), nil
	}
	var buf bytes.Buffer
	if err := encodeMessageReflect(m.ProtoReflect(), &buf); err != nil {
		return nil, err
	}
	return json.RawMessage(buf.Bytes()), nil
}
