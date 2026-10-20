package domain

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
)

// A drawing comes from an agent and is shown to people, so it is data that is not trusted. CheckArtifact is the one gate: it
// refuses anything that could run code or reach the network, requires the shape the viewer needs, and refuses a drawing whose
// words are not the words of the model it was made from. What passes is stored; what does not is kept as a failed attempt.

// ArtifactInfo is what the check learned about a drawing that passed.
type ArtifactInfo struct {
	ViewBox string
	Title   string
	Desc    string
	// ForeignWords is the share of the visible words that are not in the model or the legend (0 to 1).
	ForeignWords float64
}

const (
	// maxForeignWordShare is how much visible text may be words the model does not have. The figure is a guess, to be measured on
	// real drawings (spike S2/S4); it is a guard against a drawing of something else, not against a typo.
	maxForeignWordShare = 0.05
	minForeignWords     = 2
	maxLegendWords      = 60
	maxViewBoxSide      = 10000
)

// blockedTags can run code, load something, or break out of the page.
var blockedTags = map[string]bool{
	"script": true, "iframe": true, "frame": true, "frameset": true, "object": true, "embed": true, "applet": true, "base": true,
	"form": true, "input": true, "button": true, "textarea": true, "select": true, "option": true, "foreignobject": true, "image": true,
	"img": true, "audio": true, "video": true, "source": true, "track": true, "canvas": true, "portal": true, "link": true,
	"animate": true, "animatemotion": true, "animatetransform": true, "set": true, "feimage": true, "noscript": true, "template": true,
}

// linkAttrs hold a reference; the only reference a drawing may have is to something inside itself.
var linkAttrs = map[string]bool{
	"href": true, "xlink:href": true, "src": true, "srcset": true, "action": true, "formaction": true, "data": true, "poster": true,
	"background": true, "ping": true, "cite": true, "longdesc": true, "usemap": true,
}

var (
	cssURLRe      = regexp.MustCompile(`(?i)url\s*\(\s*([^)]*)\)`)
	cssBlockedRe  = regexp.MustCompile(`(?i)@import|expression\s*\(|javascript:|vbscript:|behavior\s*:|-moz-binding|@namespace`)
	cssCommentRe  = regexp.MustCompile(`(?s)/\*.*?\*/`)
	wordRe        = regexp.MustCompile(`[\p{L}\p{N}]+`)
	legendWordSet = legendVocabulary()
)

// legendVocabulary is what a drawing may say that the model does not: the words of the legend and the kinds. English and
// Vietnamese, because the profile language picks which one a drawing uses.
func legendVocabulary() map[string]bool {
	words := `legend added new modified changed removed deleted unchanged same deviation missing unmapped designed design delta code sad text kg
		only not in the of and or to from by service app datastore queue agent external container system systems component components
		before after base head more advisory unverified reviewed approved needs review state flow architecture behavior data
		thêm mới sửa đổi xoá bỏ không đổi lệch thiếu chưa ánh xạ thiết kế mã chỉ có trong của và hoặc đến từ dịch vụ ứng dụng
		kho dữ liệu hàng đợi tác nhân bên ngoài hệ thống trước sau thêm nữa tư vấn chưa kiểm chứng duyệt cần`
	out := map[string]bool{}
	for _, w := range wordRe.FindAllString(strings.ToLower(words), -1) {
		out[w] = true
	}
	return out
}

func unsafe(format string, a ...any) error { return ErrReviewArtifactUnsafe(fmt.Sprintf(format, a...)) }

// CheckArtifact validates the drawing of one model.
func CheckArtifact(content []byte, m DiagramModel) (ArtifactInfo, error) {
	if len(content) == 0 {
		return ArtifactInfo{}, unsafe("the drawing is empty")
	}
	if len(content) > MaxReviewArtifactBytes {
		return ArtifactInfo{}, ErrReviewArtifactTooLarge(len(content), MaxReviewArtifactBytes)
	}
	if !utf8.Valid(content) || bytes.IndexByte(content, 0) >= 0 {
		return ArtifactInfo{}, unsafe("the drawing is not text")
	}
	st := &artifactScan{allowed: modelWords(m)}
	if err := st.scan(content); err != nil {
		return ArtifactInfo{}, err
	}
	if !st.sawSVG {
		return ArtifactInfo{}, unsafe("there is no <svg> in the drawing")
	}
	if st.viewBox == "" {
		return ArtifactInfo{}, unsafe("the <svg> has no viewBox")
	}
	if st.title == "" || st.desc == "" {
		return ArtifactInfo{}, unsafe("the <svg> needs a <title> and a <desc>")
	}
	if st.legendWords > maxLegendWords {
		return ArtifactInfo{}, unsafe("the legend has %d words; the limit is %d", st.legendWords, maxLegendWords)
	}
	info := ArtifactInfo{ViewBox: st.viewBox, Title: clipRunes(st.title, 160), Desc: clipRunes(st.desc, 400)}
	if st.totalWords > 0 {
		info.ForeignWords = float64(len(st.foreign)) / float64(st.totalWords)
	}
	if len(st.foreign) >= minForeignWords && info.ForeignWords > maxForeignWordShare {
		return info, unsafe("the drawing says things the model does not (%s): it may be a drawing of something else", strings.Join(firstStrings(sortedSet(st.foreign), 6), ", "))
	}
	return info, nil
}

type artifactScan struct {
	allowed     map[string]bool
	sawSVG      bool
	viewBox     string
	title, desc string
	totalWords  int
	legendWords int
	foreign     map[string]bool

	svgDepth    int
	textDepth   int
	legendDepth int
	capture     string // "title" or "desc" while inside it
	captureBuf  strings.Builder
	styleDepth  bool
	stack       []string
	legendAt    []int
}

func (s *artifactScan) scan(content []byte) error {
	z := html.NewTokenizer(bytes.NewReader(content))
	s.foreign = map[string]bool{}
	for {
		switch z.Next() {
		case html.ErrorToken:
			if err := z.Err(); err != nil && err != io.EOF {
				return unsafe("the drawing cannot be read: %v", err)
			}
			return nil
		case html.StartTagToken, html.SelfClosingTagToken:
			tok := z.Token()
			if err := s.start(tok, tok.Type == html.SelfClosingTagToken); err != nil {
				return err
			}
		case html.EndTagToken:
			s.end(z.Token().Data)
		case html.TextToken:
			if err := s.text(string(z.Text())); err != nil {
				return err
			}
		case html.CommentToken:
			if strings.Contains(strings.ToLower(string(z.Text())), "<script") {
				return unsafe("a comment hides a script")
			}
		case html.DoctypeToken:
		}
	}
}

func (s *artifactScan) start(tok html.Token, selfClosing bool) error {
	name := strings.ToLower(tok.Data)
	if blockedTags[name] {
		return unsafe("<%s> is not allowed", name)
	}
	if name == "meta" {
		for _, a := range tok.Attr {
			if strings.EqualFold(a.Key, "http-equiv") || strings.EqualFold(a.Key, "content") && strings.Contains(strings.ToLower(a.Val), "url=") {
				return unsafe("<meta %s> is not allowed", a.Key)
			}
		}
	}
	hasLegend := false
	for _, a := range tok.Attr {
		key := strings.ToLower(a.Key)
		val := a.Val
		switch {
		case strings.HasPrefix(key, "on"):
			return unsafe("the event attribute %s is not allowed", key)
		case key == "srcdoc":
			return unsafe("srcdoc is not allowed")
		case linkAttrs[key]:
			if v := strings.TrimSpace(val); v != "" && !strings.HasPrefix(v, "#") {
				return unsafe("%s=%q points outside the drawing; only #fragments are allowed", key, clipRunes(val, 40))
			}
		case key == "style":
			if err := checkCSS(val); err != nil {
				return err
			}
		case key == "id":
			if !strings.HasPrefix(val, ReviewArtifactIDPrefix) {
				return unsafe("id %q does not start with %s", clipRunes(val, 40), ReviewArtifactIDPrefix)
			}
			if strings.HasPrefix(val, ReviewArtifactIDPrefix+"legend") {
				hasLegend = true
			}
		}
	}
	if name == "style" {
		s.styleDepth = true
	}
	if name == "svg" {
		s.svgDepth++
		if !s.sawSVG {
			s.sawSVG = true
			for _, a := range tok.Attr {
				if strings.EqualFold(a.Key, "viewBox") {
					vb, err := parseViewBox(a.Val)
					if err != nil {
						return err
					}
					s.viewBox = vb
				}
			}
		}
	}
	if selfClosing {
		return nil
	}
	if s.svgDepth > 0 && (name == "title" || name == "desc") && s.capture == "" && ((name == "title" && s.title == "") || (name == "desc" && s.desc == "")) {
		s.capture = name
		s.captureBuf.Reset()
	}
	if name == "text" || name == "tspan" {
		s.textDepth++
	}
	s.stack = append(s.stack, name)
	if hasLegend {
		s.legendDepth++
		s.legendAt = append(s.legendAt, len(s.stack))
	} else {
		s.legendAt = append(s.legendAt, 0)
	}
	return nil
}

func (s *artifactScan) end(name string) {
	name = strings.ToLower(name)
	if name == s.capture {
		text := strings.Join(strings.Fields(s.captureBuf.String()), " ")
		if name == "title" {
			s.title = text
		} else {
			s.desc = text
		}
		s.capture = ""
	}
	if name == "style" {
		s.styleDepth = false
	}
	if name == "svg" && s.svgDepth > 0 {
		s.svgDepth--
	}
	if name == "text" || name == "tspan" {
		if s.textDepth > 0 {
			s.textDepth--
		}
	}
	for i := len(s.stack) - 1; i >= 0; i-- { // pop to the matching open tag; malformed nesting is tolerated
		if s.stack[i] == name {
			for j := len(s.stack) - 1; j >= i; j-- {
				if s.legendAt[j] != 0 {
					s.legendDepth--
				}
			}
			s.stack, s.legendAt = s.stack[:i], s.legendAt[:i]
			break
		}
	}
}

func (s *artifactScan) text(t string) error {
	if s.styleDepth {
		return checkCSS(t)
	}
	if s.capture != "" {
		s.captureBuf.WriteString(t)
	}
	if s.textDepth == 0 {
		return nil
	}
	for _, w := range wordRe.FindAllString(strings.ToLower(t), -1) {
		s.totalWords++
		if s.legendDepth > 0 {
			s.legendWords++
			continue
		}
		if !s.allowed[w] && !legendWordSet[w] && !isNumber(w) {
			s.foreign[w] = true
		}
	}
	return nil
}

func isNumber(w string) bool {
	for _, r := range w {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return w != ""
}

func parseViewBox(v string) (string, error) {
	parts := strings.Fields(strings.ReplaceAll(v, ",", " "))
	if len(parts) != 4 {
		return "", unsafe("viewBox %q is not four numbers", clipRunes(v, 40))
	}
	nums := make([]float64, 4)
	for i, p := range parts {
		f, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return "", unsafe("viewBox %q is not four numbers", clipRunes(v, 40))
		}
		nums[i] = f
	}
	if nums[2] <= 0 || nums[3] <= 0 || nums[2] > maxViewBoxSide || nums[3] > maxViewBoxSide {
		return "", unsafe("viewBox %q has a side outside 1 to %d", clipRunes(v, 40), maxViewBoxSide)
	}
	return strings.Join(parts, " "), nil
}

// checkCSS refuses what can load or run something. A url() may point at a #fragment of the drawing and nothing else.
func checkCSS(css string) error {
	css = cssCommentRe.ReplaceAllString(css, " ")
	if m := cssBlockedRe.FindString(css); m != "" {
		return unsafe("the style uses %q", strings.TrimSpace(m))
	}
	for _, m := range cssURLRe.FindAllStringSubmatch(css, -1) {
		arg := strings.Trim(strings.TrimSpace(m[1]), `"'`)
		if !strings.HasPrefix(strings.TrimSpace(arg), "#") {
			return unsafe("the style loads %q; only #fragments are allowed", clipRunes(arg, 40))
		}
	}
	return nil
}

// modelWords are the words a drawing of the model may use.
func modelWords(m DiagramModel) map[string]bool {
	out := map[string]bool{}
	add := func(s string) {
		for _, w := range wordRe.FindAllString(strings.ToLower(s), -1) {
			out[w] = true
		}
	}
	add(m.Name)
	for _, n := range m.Nodes {
		add(n.Label)
		add(n.Kind)
		add(n.Tech)
		add(n.ID)
		add(n.KGID)
		for _, c := range n.Columns {
			add(c.Name)
			add(c.Type)
		}
	}
	for _, e := range m.Edges {
		add(e.Label)
		add(e.Protocol)
	}
	return out
}

func sortedSet(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func firstStrings(in []string, n int) []string {
	if len(in) > n {
		return in[:n]
	}
	return in
}
