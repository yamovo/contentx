package services

import (
	"strings"
	"testing"
)

// 合法 SVG 基线，用于多个用例。
const cleanSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
  <rect x="10" y="10" width="80" height="80" fill="blue"/>
  <circle cx="50" cy="50" r="20" fill="red"/>
  <text x="50" y="55" text-anchor="middle">Hi</text>
</svg>`

func TestSanitizeSVG_RemovesScriptElement(t *testing.T) {
	in := `<svg xmlns="http://www.w3.org/2000/svg"><script>alert('xss')</script><rect width="10" height="10"/></svg>`
	out, err := SanitizeSVG([]byte(in))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(out)
	if strings.Contains(strings.ToLower(s), "<script") {
		t.Errorf("script element not removed: %s", s)
	}
	if !strings.Contains(s, "<rect") {
		t.Errorf("rect should be preserved: %s", s)
	}
}

func TestSanitizeSVG_RemovesEventHandlers(t *testing.T) {
	in := `<svg xmlns="http://www.w3.org/2000/svg"><rect onclick="evil()" onload="evil()" width="10" height="10"/></svg>`
	out, err := SanitizeSVG([]byte(in))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := strings.ToLower(string(out))
	for _, attr := range []string{"onclick", "onload", "onerror", "onmouseover"} {
		if strings.Contains(s, attr) {
			t.Errorf("event handler %q not removed: %s", attr, s)
		}
	}
}

func TestSanitizeSVG_RemovesExternalHref(t *testing.T) {
	cases := []struct {
		name, in string
	}{
		{"use external", `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><use xlink:href="http://evil.com/x.svg#s"/></svg>`},
		{"image external", `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><image xlink:href="http://evil.com/x.png"/></svg>`},
		{"javascript href", `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><use xlink:href="javascript:alert(1)"/></svg>`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := SanitizeSVG([]byte(c.in))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			s := strings.ToLower(string(out))
			if strings.Contains(s, "http://evil.com") {
				t.Errorf("external reference not removed: %s", s)
			}
			if strings.Contains(s, "javascript:") {
				t.Errorf("javascript: reference not removed: %s", s)
			}
		})
	}
}

func TestSanitizeSVG_PreservesInternalHref(t *testing.T) {
	in := `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><defs><rect id="r" width="10" height="10"/></defs><use xlink:href="#r"/></svg>`
	out, err := SanitizeSVG([]byte(in))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(out), "#r") {
		t.Errorf("internal reference removed: %s", out)
	}
}

func TestSanitizeSVG_RemovesForeignObject(t *testing.T) {
	in := `<svg xmlns="http://www.w3.org/2000/svg"><foreignObject><body><script>alert(1)</script></body></foreignObject><rect width="10" height="10"/></svg>`
	out, err := SanitizeSVG([]byte(in))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := strings.ToLower(string(out))
	if strings.Contains(s, "foreignobject") {
		t.Errorf("foreignObject not removed: %s", s)
	}
	if strings.Contains(s, "<script") {
		t.Errorf("script inside foreignObject not removed: %s", s)
	}
}

func TestSanitizeSVG_RemovesStyleAttributeThreats(t *testing.T) {
	cases := []struct {
		name, in, bad string
	}{
		{"javascript in style", `<svg xmlns="http://www.w3.org/2000/svg"><rect style="fill: javascript:alert(1)" width="10" height="10"/></svg>`, "javascript:"},
		{"expression in style", `<svg xmlns="http://www.w3.org/2000/svg"><rect style="width: expression(alert(1))" width="10" height="10"/></svg>`, "expression("},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := SanitizeSVG([]byte(c.in))
			if err != nil {
				// 净化后整体拒绝也可接受
				return
			}
			if strings.Contains(strings.ToLower(string(out)), c.bad) {
				t.Errorf("dangerous style not removed: %s", out)
			}
		})
	}
}

func TestSanitizeSVG_RemovesComments(t *testing.T) {
	in := `<svg xmlns="http://www.w3.org/2000/svg"><!-- secret --><rect width="10" height="10"/></svg>`
	out, err := SanitizeSVG([]byte(in))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(string(out), "secret") {
		t.Errorf("comment content not removed: %s", out)
	}
}

func TestSanitizeSVG_RemovesDOCTYPE(t *testing.T) {
	in := `<?xml version="1.0"?><!DOCTYPE svg [<!ENTITY xxe SYSTEM "file:///etc/passwd">]><svg xmlns="http://www.w3.org/2000/svg"><rect width="10" height="10"/></svg>`
	out, err := SanitizeSVG([]byte(in))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := strings.ToLower(string(out))
	if strings.Contains(s, "doctype") {
		t.Errorf("DOCTYPE not removed: %s", s)
	}
	if strings.Contains(s, "entity") {
		t.Errorf("entity declaration not removed: %s", s)
	}
}

func TestSanitizeSVG_PreservesCleanSVG(t *testing.T) {
	out, err := SanitizeSVG([]byte(cleanSVG))
	if err != nil {
		t.Fatalf("clean SVG should pass: %v", err)
	}
	s := string(out)
	for _, want := range []string{"<svg", "<rect", "<circle", "<text", "viewBox"} {
		if !strings.Contains(s, want) {
			t.Errorf("expected %q in output: %s", want, s)
		}
	}
}

func TestSanitizeSVG_RejectsNonXML(t *testing.T) {
	in := []byte("this is not svg at all <svg")
	if _, err := SanitizeSVG(in); err == nil {
		t.Errorf("expected error for non-XML input")
	}
}

func TestSanitizeSVG_HandlesUppercaseScriptTag(t *testing.T) {
	// XML 大小写敏感，<SCRIPT> 不在白名单会被移除；输出中不应残留任何 script 元素。
	in := `<svg xmlns="http://www.w3.org/2000/svg"><SCRIPT>alert(1)</SCRIPT><rect width="10" height="10"/></svg>`
	out, err := SanitizeSVG([]byte(in))
	if err != nil {
		return // 整体拒绝也可接受
	}
	s := strings.ToLower(string(out))
	if strings.Contains(s, "<script") {
		t.Errorf("uppercase script tag not removed: %s", s)
	}
	if !strings.Contains(s, "<rect") {
		t.Errorf("rect should be preserved: %s", s)
	}
}
