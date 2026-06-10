// Package pathutil 在代码生成阶段将 Google API HTTP 路径模板转换为 Fiber v3 路由。
//
// 示例：
//
//	/v1/books/{id}           -> /v1/books/:id
//	/v1/books/{book.id}      -> /v1/books/:book_id  (+ SetURIParam 回填嵌套字段)
//	/v1/{name=projects/*}/x  -> /v1/:name<regex(...)>/x
package pathutil

import (
	"fmt"
	"regexp"
	"strings"
)

var pathVarRE = regexp.MustCompile(`(?i){([a-z.0-9_\s]*)=?([^{}]*)}`)

// ParamBinding maps a Fiber route parameter to proto field path.
type ParamBinding struct {
	ParamName string
	FieldPath string
}

// Convert transforms a Google API HTTP path template into a Fiber v3 route.
func Convert(pathTemplate string) (fiberPath string, params []ParamBinding) {
	fiberPath = pathTemplate
	matches := pathVarRE.FindAllStringSubmatch(pathTemplate, -1)
	seen := make(map[string]struct{}, len(matches))
	for _, m := range matches {
		fieldPath := strings.TrimSpace(m[1])
		if fieldPath == "" {
			continue
		}
		pattern := strings.TrimSpace(m[2])
		paramName := toParamName(fieldPath)
		replacement := ":" + paramName
		if pattern != "" {
			replacement += fiberConstraint(pattern)
		}
		old := m[0]
		fiberPath = strings.Replace(fiberPath, old, replacement, 1)
		if _, ok := seen[paramName]; ok {
			continue
		}
		seen[paramName] = struct{}{}
		params = append(params, ParamBinding{
			ParamName: paramName,
			FieldPath: fieldPath,
		})
	}
	return fiberPath, params
}

func toParamName(fieldPath string) string {
	return strings.ReplaceAll(fieldPath, ".", "_")
}

func fiberConstraint(pattern string) string {
	segs := strings.Split(pattern, "/")
	for i, seg := range segs {
		switch seg {
		case "*":
			segs[i] = "[^/]+"
		case "**":
			segs[i] = ".*"
		default:
			segs[i] = regexp.QuoteMeta(seg)
		}
	}
	expr := strings.Join(segs, "/")
	return fmt.Sprintf("<regex(^%s$)>", expr)
}

func camelCase(s string) string {
	if s == "" {
		return ""
	}
	t := make([]byte, 0, 32)
	i := 0
	if s[0] == '_' {
		t = append(t, 'X')
		i++
	}
	for ; i < len(s); i++ {
		c := s[i]
		if c == '_' && i+1 < len(s) && isLower(s[i+1]) {
			continue
		}
		if c >= '0' && c <= '9' {
			t = append(t, c)
			continue
		}
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		t = append(t, c)
		for i+1 < len(s) && s[i+1] >= 'a' && s[i+1] <= 'z' {
			i++
			t = append(t, s[i])
		}
	}
	return string(t)
}

func isLower(c byte) bool { return c >= 'a' && c <= 'z' }
