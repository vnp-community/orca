package domain

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ReadinessHints is the data already on hand for suggested defaults (never a model call).
type ReadinessHints struct {
	Title       string
	Urgency     Urgency
	Size        RequestSize
	SourceHints SourceHints
	// AIDraft is an optional AC draft filled by the opt-in REQUEST_READINESS_AI_DRAFT step.
	AIDraft string
}

type QuestionBuilder struct{}

// Build turns blocking gaps into questions in the order the report lists them. Non-blocking gaps ask nothing.
func (QuestionBuilder) Build(report ReadinessReport, hints ReadinessHints) []ClarificationQuestion {
	t := report.Type
	rules := map[string]FieldRule{}
	for _, r := range RequiredFields(t) {
		rules[r.Path()] = r
	}
	var out []ClarificationQuestion
	seen := map[string]bool{}
	for _, m := range report.Blocker() {
		path := m.Path
		if strings.HasPrefix(path, "/acceptance_criteria") {
			path = "/acceptance_criteria"
		}
		if seen[path] {
			continue
		}
		seen[path] = true
		var q ClarificationQuestion
		switch {
		case path == "/title":
			q = textQuestion("title", "title", "Tiêu đề của yêu cầu là gì?", "Yêu cầu chưa có tiêu đề")
		case path == "/body":
			q = textQuestion("body", "body", "Hãy mô tả chi tiết vấn đề hoặc yêu cầu (ít nhất 20 ký tự).", "Mô tả hiện quá ngắn để phân tích")
		case path == "/acceptance_criteria":
			q = textQuestion("acceptance_criteria", "acceptance_criteria", "Tiêu chí chấp nhận: mỗi dòng một tiêu chí, kiểm chứng được.", "Cần ít nhất một tiêu chí để biết khi nào yêu cầu hoàn thành")
			q.SuggestedDefault = suggestACDefault(hints)
		case rules[path].Key != "":
			q = fieldQuestion(t, rules[path], hints)
		default:
			continue
		}
		q.Seq = len(out) + 1
		q.Required = true
		out = append(out, q)
	}
	return out
}

func textQuestion(key, target, prompt, reason string) ClarificationQuestion {
	return ClarificationQuestion{QuestionKey: key, Kind: QuestionKindText, Prompt: prompt, Reason: reason, TargetPath: target}
}

func fieldQuestion(t RequestType, r FieldRule, hints ReadinessHints) ClarificationQuestion {
	key := "type_fields." + r.Key
	reason := fmt.Sprintf("Loại %s cần trường %s để tiếp tục", t, r.Key)
	q := ClarificationQuestion{QuestionKey: key, TargetPath: key, Reason: reason}
	switch r.Kind {
	case FieldEnum:
		q.Kind = QuestionKindSingleChoice
		q.Prompt = fieldPrompt(r.Key)
		for _, v := range r.Enum {
			q.Options = append(q.Options, QuestionOption{ID: v, Label: v})
		}
		q.SuggestedDefault = suggestEnumDefault(r, hints)
	case FieldBool:
		q.Kind = QuestionKindBoolean
		q.Prompt = fieldPrompt(r.Key)
	case FieldList:
		q.Kind = QuestionKindText
		q.Prompt = fieldPrompt(r.Key) + " (mỗi dòng một mục)"
	default:
		q.Kind = QuestionKindText
		q.Prompt = fieldPrompt(r.Key)
	}
	return q
}

var fieldPrompts = map[string]string{
	"repro_steps":         "Các bước tái hiện lỗi",
	"actual":              "Kết quả thực tế",
	"expected":            "Kết quả mong đợi",
	"environment":         "Môi trường xảy ra (phiên bản, hệ điều hành, cấu hình)",
	"severity":            "Mức độ nghiêm trọng",
	"goal":                "Mục tiêu của thay đổi",
	"value":               "Giá trị mang lại",
	"scope_in":            "Phạm vi bao gồm",
	"scope_out":           "Phạm vi loại trừ",
	"affected_components": "Thành phần bị ảnh hưởng",
	"exploitability":      "Khả năng khai thác",
	"data_exposed":        "Dữ liệu có bị lộ không?",
	"metric":              "Chỉ số cần cải thiện",
	"current_value":       "Giá trị hiện tại",
	"target_value":        "Giá trị mục tiêu",
	"unit":                "Đơn vị",
	"target_environment":  "Môi trường đích",
	"window":              "Khung thời gian thực hiện",
	"rollback_plan":       "Kế hoạch hoàn tác",
	"production_impact":   "Tác động lên production",
	"started_at":          "Thời điểm bắt đầu sự cố",
	"question":            "Câu hỏi cần trả lời",
	"time_box_hours":      "Giới hạn thời gian (giờ)",
	"audience":            "Đối tượng đọc tài liệu",
	"scope":               "Phạm vi tài liệu",
}

func fieldPrompt(key string) string {
	if p, ok := fieldPrompts[key]; ok {
		return p
	}
	return key
}

func suggestACDefault(h ReadinessHints) json.RawMessage {
	if h.AIDraft != "" {
		b, _ := json.Marshal(h.AIDraft)
		return b
	}
	if strings.TrimSpace(h.Title) == "" {
		return nil
	}
	b, _ := json.Marshal("Yêu cầu \"" + strings.TrimSpace(h.Title) + "\" được đáp ứng và kiểm chứng được")
	return b
}

// suggestEnumDefault maps the issue priority and urgency onto severity-like enums; anything else suggests nothing.
func suggestEnumDefault(r FieldRule, h ReadinessHints) json.RawMessage {
	if r.Key != "severity" && r.Key != "exploitability" {
		return nil
	}
	pick := "medium"
	switch strings.ToLower(strings.TrimSpace(h.SourceHints.Priority)) {
	case "highest", "critical", "blocker":
		pick = "critical"
	case "high", "major":
		pick = "high"
	case "low", "lowest", "minor", "trivial":
		pick = "low"
	}
	if h.Urgency == UrgencyUrgent && (pick == "medium" || pick == "low") {
		pick = "high"
	}
	if !containsString(r.Enum, pick) {
		// exploitability has no "critical": clamp to the top of its scale
		pick = r.Enum[len(r.Enum)-1]
	}
	b, _ := json.Marshal(pick)
	return b
}
