package services

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// svgAllowedElements 是 SVG 元素白名单。
// 不在列表中的元素（script、foreignObject、image 等）会被连同内容一起移除。
var svgAllowedElements = map[string]bool{
	"svg": true, "g": true, "defs": true, "symbol": true,
	"path": true, "rect": true, "circle": true, "ellipse": true,
	"line": true, "polyline": true, "polygon": true,
	"text": true, "tspan": true, "textPath": true,
	"linearGradient": true, "radialGradient": true, "stop": true,
	"pattern": true, "clipPath": true, "mask": true,
	"title": true, "desc": true,
	"filter": true, "feGaussianBlur": true, "feOffset": true,
	"feMerge": true, "feMergeNode": true, "feColorMatrix": true,
	"feFlood": true, "feComposite": true, "feBlend": true,
	"feComponentTransfer": true, "feFuncR": true, "feFuncG": true,
	"feFuncB": true, "feFuncA": true, "feTile": true, "feTurbulence": true,
	"feDisplacementMap": true, "feMorphology": true, "feConvolveMatrix": true,
	"feSpecularLighting": true, "feDiffuseLighting": true,
	"feDistantLight": true, "fePointLight": true, "feSpotLight": true,
	"marker": true, "use": true, "switch": true,
	"animate": true, "animateMotion": true, "animateTransform": true,
	"set": true, "mpath": true,
}

// svgAllowedAttrs 是 SVG 属性白名单。
// 所有 on* 事件处理器（onclick、onload、onerror……）因不在列表中而被自动移除。
var svgAllowedAttrs = map[string]bool{
	"id": true, "class": true, "lang": true, "dir": true, "tabindex": true,
	"version": true, "xmlns": true, "xmlns:xlink": true,
	"xml:space": true, "xml:lang": true,
	"viewBox": true, "preserveAspectRatio": true,
	"x": true, "y": true, "width": true, "height": true,
	"cx": true, "cy": true, "r": true, "rx": true, "ry": true,
	"x1": true, "y1": true, "x2": true, "y2": true,
	"points": true, "d": true,
	"fill": true, "fill-rule": true, "fill-opacity": true,
	"stroke": true, "stroke-width": true, "stroke-linecap": true,
	"stroke-linejoin": true, "stroke-dasharray": true,
	"stroke-dashoffset": true, "stroke-opacity": true,
	"stroke-miterlimit": true,
	"opacity": true, "transform": true,
	"font-family": true, "font-size": true, "font-weight": true,
	"font-style": true, "font-variant": true, "text-decoration": true,
	"text-anchor": true, "letter-spacing": true, "word-spacing": true,
	"writing-mode": true, "baseline-shift": true, "dominant-baseline": true,
	"gradientUnits": true, "gradientTransform": true,
	"fx": true, "fy": true,
	"offset": true, "stop-color": true, "stop-opacity": true,
	"href": true, "xlink:href": true, // 值在下方校验
	"clip-path": true, "mask": true, "filter": true,
	"marker-start": true, "marker-mid": true, "marker-end": true,
	"markerUnits": true, "markerWidth": true, "markerHeight": true,
	"refX": true, "refY": true, "orient": true,
	"style": true, // 值在下方校验
	"spreadMethod": true,
	"patternUnits": true, "patternTransform": true,
	"clipPathUnits": true, "maskUnits": true, "maskContentUnits": true,
	"patternContentUnits": true,
	"systemLanguage": true, "requiredFeatures": true,
	"attributeName": true, "attributeType": true,
	"from": true, "to": true, "by": true, "values": true,
	"begin": true, "end": true, "dur": true, "repeatCount": true,
	"repeatDur": true, "restart": true, "calcMode": true,
	"keyTimes": true, "keySplines": true, "keyPoints": true,
	"rotate": true, "path": true, "type": true,
	"mode": true, "in": true, "in2": true, "result": true,
	"operator": true, "k1": true, "k2": true, "k3": true, "k4": true,
	"tableValues": true, "slope": true, "intercept": true,
	"amplitude": true, "exponent": true,
	"baseFrequency": true, "numOctaves": true, "seed": true,
	"stitchTiles": true,
	"xChannelSelector": true, "yChannelSelector": true,
	"scale": true, "dx": true, "dy": true,
	"divisor": true, "kernelMatrix": true, "kernelUnitLength": true,
	"targetX": true, "targetY": true, "edgeMode": true,
	"preserveAlpha": true, "order": true,
	"radius": true, "azimuth": true, "elevation": true,
	"pointsAtX": true, "pointsAtY": true, "pointsAtZ": true,
	"specularExponent": true, "specularConstant": true,
	"surfaceScale": true, "diffuseConstant": true,
	"limitingConeAngle": true,
}

// SanitizeSVG 解析 SVG 内容并返回净化后的字节切片。
//
// 移除内容：
//   - <script>、<foreignObject>、<image> 及任何不在白名单中的元素
//   - 所有 on* 事件处理器属性（onclick、onload、onerror……）
//   - 外部 href / xlink:href 引用（仅允许内部 "#id" 引用）
//   - style 属性中的 javascript:、expression(、vbscript:、-moz-binding
//   - XML 注释与 DOCTYPE 声明（XXE 防护）
//
// 若输入不是合法 XML，返回错误。
func SanitizeSVG(data []byte) ([]byte, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	dec.Entity = nil // 禁用自定义实体（XXE 防护）

	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)

	skipDepth := 0

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("invalid SVG XML: %w", err)
		}

		// 正在跳过非白名单元素时，仅跟踪嵌套深度。
		if skipDepth > 0 {
			switch tok.(type) {
			case xml.StartElement:
				skipDepth++
			case xml.EndElement:
				skipDepth--
			}
			continue
		}

		switch t := tok.(type) {
		case xml.ProcInst:
			// 允许 <?xml ...?> 与 <?xml-stylesheet?>。
			if t.Target == "xml" || t.Target == "xml-stylesheet" {
				_ = enc.EncodeToken(t)
			}

		case xml.Directive:
			// 丢弃 DOCTYPE 等（XXE 防护）。

		case xml.Comment:
			// 丢弃注释（防止条件注释攻击）。

		case xml.StartElement:
			if !svgAllowedElements[t.Name.Local] {
				skipDepth = 1
				continue
			}
			t.Attr = filterSVGAttrs(t.Attr)
			_ = enc.EncodeToken(t)

		case xml.EndElement:
			if svgAllowedElements[t.Name.Local] {
				_ = enc.EncodeToken(t)
			}

		case xml.CharData:
			_ = enc.EncodeToken(t)

		default:
			_ = enc.EncodeToken(tok)
		}
	}

	if err := enc.Flush(); err != nil {
		return nil, err
	}

	sanitized := buf.Bytes()

	// 最终安全扫描：若仍残留危险子串则整体拒绝。
	lower := strings.ToLower(string(sanitized))
	if strings.Contains(lower, "javascript:") ||
		strings.Contains(lower, "vbscript:") ||
		strings.Contains(lower, "expression(") ||
		strings.Contains(lower, "<script") ||
		strings.Contains(lower, "<foreignobject") {
		return nil, fmt.Errorf("SVG contains potentially malicious content")
	}

	return sanitized, nil
}

// filterSVGAttrs 移除元素上的危险属性。
func filterSVGAttrs(attrs []xml.Attr) []xml.Attr {
	result := make([]xml.Attr, 0, len(attrs))
	for _, attr := range attrs {
		local := attr.Name.Local

		// 阻断所有 on* 事件处理器。
		if strings.HasPrefix(local, "on") && len(local) > 2 {
			continue
		}

		// 阻断不在白名单中的属性。
		if !svgAllowedAttrs[local] {
			continue
		}

		val := attr.Value
		lower := strings.ToLower(val)

		// href / xlink:href：仅允许内部引用（"#..."）。
		if local == "href" || local == "xlink:href" {
			if !strings.HasPrefix(strings.TrimSpace(val), "#") {
				continue
			}
		}

		// style：阻断危险 CSS 模式。
		if local == "style" {
			if strings.Contains(lower, "javascript:") ||
				strings.Contains(lower, "vbscript:") ||
				strings.Contains(lower, "expression(") ||
				strings.Contains(lower, "-moz-binding") {
				continue
			}
		}

		result = append(result, attr)
	}
	return result
}
