package domain

import (
	"fmt"
	"strings"
	"testing"
)

// drawingOf writes the kind of page the skill returns: a self-contained HTML file with one inline SVG, a style block that uses
// system fonts, ids with the Orca prefix, and one text per label of the model.
func drawingOf(m DiagramModel, extraSVG string) []byte {
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width">`)
	b.WriteString(`<title>Architecture</title><style>:root{--accent:#d97757}svg{font-family:ui-sans-serif,system-ui,sans-serif}.box{fill:url(#rv-grad);stroke:#333}</style></head><body>`)
	b.WriteString(`<svg id="rv-arch" viewBox="0 0 960 540" xmlns="http://www.w3.org/2000/svg"><title id="rv-t">Container view</title><desc id="rv-d">Containers and their dependencies</desc>`)
	b.WriteString(`<defs><linearGradient id="rv-grad"><stop offset="0" stop-color="#fff"/></linearGradient></defs>`)
	for i, n := range m.Nodes {
		fmt.Fprintf(&b, `<g id="rv-n%d"><rect class="box" x="%d" y="40" width="120" height="50"/><text x="%d" y="60">%s</text><text x="%d" y="78">[%s: %s]</text></g>`, i, 20+i*140, 30+i*140, n.Label, 30+i*140, n.Kind, n.Tech)
	}
	for _, e := range m.Edges {
		fmt.Fprintf(&b, `<g><path d="M0 0L10 10" stroke="#333"/><text x="5" y="5">%s %s</text></g>`, e.Protocol, e.Label)
	}
	b.WriteString(`<g id="rv-legend"><text x="10" y="500">Legend: added, removed, deviation</text></g>`)
	b.WriteString(extraSVG)
	b.WriteString(`</svg></body></html>`)
	return []byte(b.String())
}

func designModel(t *testing.T) DiagramModel {
	t.Helper()
	models, err := BuildDesignDiagrams(lockedPilotDesign(t))
	if err != nil || len(models) == 0 {
		t.Fatal(err)
	}
	return models[0]
}

func TestCheckArtifact_AcceptsAWellFormedDrawingAndReadsItsFacts(t *testing.T) {
	m := designModel(t)
	info, err := CheckArtifact(drawingOf(m, ""), m)
	if err != nil {
		t.Fatal(err)
	}
	if info.ViewBox != "0 0 960 540" || info.Title != "Container view" || info.Desc != "Containers and their dependencies" || info.ForeignWords != 0 {
		t.Errorf("info: %+v", info)
	}
}

func TestCheckArtifact_RefusesWhatCouldRunOrReachOut(t *testing.T) {
	m := designModel(t)
	hostile := map[string]string{
		"script":                     `<script>alert(1)</script>`,
		"script in svg":              `<g id="rv-x"><script>fetch('//evil')</script></g>`,
		"event handler":              `<rect id="rv-y" onclick="x()" width="1" height="1"/>`,
		"event handler on svg":       `<g id="rv-y" onLoad="x()"></g>`,
		"javascript link":            `<a id="rv-a" href="javascript:alert(1)"><text>x</text></a>`,
		"obfuscated javascript link": `<a id="rv-a" href="java&#x73;cript:alert(1)"><text>x</text></a>`,
		"data link":                  `<a id="rv-a" href="data:text/html;base64,PHNjcmlwdD4="><text>x</text></a>`,
		"external link":              `<a id="rv-a" href="https://evil.example/x"><text>x</text></a>`,
		"external use":               `<use id="rv-u" href="https://evil.example/s.svg#x"/>`,
		"xlink href":                 `<use id="rv-u" xlink:href="//evil.example/s.svg#x"/>`,
		"image":                      `<image id="rv-i" href="#x" width="1" height="1"/>`,
		"iframe":                     `<iframe src="about:blank"></iframe>`,
		"object":                     `<object data="x"></object>`,
		"embed":                      `<embed src="x">`,
		"foreignObject":              `<foreignObject id="rv-f"><div>hi</div></foreignObject>`,
		"form":                       `<form action="/x"><input name="a"></form>`,
		"base":                       `<base href="https://evil.example/">`,
		"meta refresh":               `<meta http-equiv="refresh" content="0;url=https://evil.example/">`,
		"link stylesheet":            `<link rel="stylesheet" href="https://fonts.googleapis.com/css?family=Geist">`,
		"smil set":                   `<set attributeName="href" to="javascript:alert(1)"/>`,
		"animate":                    `<animate attributeName="x" from="0" to="9"/>`,
		"style import":               `<style>@import url("https://fonts.googleapis.com/css?family=Geist");</style>`,
		"style url":                  `<style>.a{background:url(https://evil.example/p.png)}</style>`,
		"style url in attribute":     `<rect id="rv-r" style="fill:url('//evil.example/x')" width="1" height="1"/>`,
		"style expression":           `<rect id="rv-r" style="width:expression(alert(1))" width="1" height="1"/>`,
		"style javascript":           `<rect id="rv-r" style="background:javascript:alert(1)" width="1" height="1"/>`,
		"srcdoc":                     `<g id="rv-g" srcdoc="x"></g>`,
		"comment with script":        `<!-- <script>alert(1)</script> -->`,
		"id without the prefix":      `<g id="node1"></g>`,
		"template":                   `<template><script>1</script></template>`,
	}
	for name, frag := range hostile {
		if _, err := CheckArtifact(drawingOf(m, frag), m); err == nil {
			t.Errorf("%s was accepted", name)
		} else if !strings.Contains(err.Error(), CodeReviewArtifactUnsafe) && !strings.Contains(err.Error(), "drawing was refused") {
			t.Errorf("%s: unexpected error %v", name, err)
		}
	}
	// The same page without the hostile part passes, so the refusal is because of the fragment.
	if _, err := CheckArtifact(drawingOf(m, ""), m); err != nil {
		t.Fatal(err)
	}
}

func TestCheckArtifact_FragmentsInsideTheDrawingAreFine(t *testing.T) {
	m := designModel(t)
	ok := `<use id="rv-u" href="#rv-n0"/><path id="rv-p" style="fill:url(#rv-grad);stroke:var(--accent)" d="M0 0"/><a id="rv-a" href="#rv-n1"><text>x</text></a>`
	if _, err := CheckArtifact(drawingOf(m, ok), m); err != nil {
		t.Errorf("references to the drawing itself: %v", err)
	}
}

func TestCheckArtifact_ShapeAndSize(t *testing.T) {
	m := designModel(t)
	good := string(drawingOf(m, ""))
	cases := map[string]string{
		"empty":        "",
		"not svg":      `<!doctype html><html><body><p>hello</p></body></html>`,
		"no viewBox":   strings.Replace(good, ` viewBox="0 0 960 540"`, "", 1),
		"bad viewBox":  strings.Replace(good, `0 0 960 540`, `0 0 a b`, 1),
		"huge viewBox": strings.Replace(good, `0 0 960 540`, `0 0 99999 540`, 1),
		"zero viewBox": strings.Replace(good, `0 0 960 540`, `0 0 0 0`, 1),
		"no title":     strings.Replace(good, `<title id="rv-t">Container view</title>`, ``, 1),
		"no desc":      strings.Replace(good, `<desc id="rv-d">Containers and their dependencies</desc>`, ``, 1),
		"not utf-8":    good + "\xff\xfe",
		"a nul byte":   good + "\x00",
	}
	for name, doc := range cases {
		if _, err := CheckArtifact([]byte(doc), m); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	big := drawingOf(m, strings.Repeat(`<g id="rv-pad"></g>`, MaxReviewArtifactBytes/10))
	if _, err := CheckArtifact(big, m); err == nil || !strings.Contains(err.Error(), CodeReviewArtifactTooLarge) && !strings.Contains(err.Error(), "limit is") {
		t.Errorf("too large: %v", err)
	}
}

func TestCheckArtifact_TheWordsMustBeTheModelsWords(t *testing.T) {
	m := designModel(t)
	// A drawing of something else: same shape, other words.
	other := drawingOf(m, `<g id="rv-extra"><text>Quarterly revenue forecast pipeline</text><text>Marketing funnel conversion</text></g>`)
	if _, err := CheckArtifact(other, m); err == nil || !strings.Contains(err.Error(), "model does not") {
		t.Errorf("foreign words: %v", err)
	}
	// One stray word is not enough to refuse a drawing (the share and the count both matter).
	one := drawingOf(m, `<g id="rv-extra"><text>Overview</text></g>`)
	if _, err := CheckArtifact(one, m); err != nil {
		t.Errorf("a single extra word: %v", err)
	}
	// Words of the legend and Vietnamese legend words are the drawing's own.
	legend := drawingOf(m, `<g id="rv-extra"><text>added removed deviation thêm xoá lệch</text></g>`)
	if _, err := CheckArtifact(legend, m); err != nil {
		t.Errorf("legend words: %v", err)
	}
	// A legend group is exempt but short.
	long := drawingOf(m, `<g id="rv-legend-long"><text>`+strings.Repeat("alpha ", 80)+`</text></g>`)
	if _, err := CheckArtifact(long, m); err == nil {
		t.Error("an exempt legend still has a size limit")
	}
}

func TestCheckArtifact_RandomJunkNeverPanics(t *testing.T) {
	m := designModel(t)
	for _, doc := range []string{"<", "<svg", "<svg viewBox=", "<<<<>>>>", "<svg><title>", "</svg></svg>", "<style>", "<g id=", strings.Repeat("<g>", 5000)} {
		_, _ = CheckArtifact([]byte(doc), m)
	}
}
