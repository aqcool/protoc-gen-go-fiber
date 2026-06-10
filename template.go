package main

import (
	"bytes"
	_ "embed"
	"sort"
	"strings"
	"text/template"

	"github.com/aqcool/protoc-gen-go-fiber/internal/pathutil"
)

// serviceDesc 与 methodDesc 是模板渲染用的中间结构，由 fiber.go 从 proto 解析后填充。
//
//go:embed fiberTemplate.tpl
var fiberTemplate string

type serviceDesc struct {
	ServiceType string
	ServiceName string
	Metadata    string
	Methods     []*methodDesc
	MethodSets  map[string]*methodDesc
}

type methodDesc struct {
	Name         string
	OriginalName string
	Num          int
	Request      string
	Reply        string
	Comment      string
	Path         string
	PathTemplate string
	FiberPath    string
	FiberMethod  string
	Method       string
	HasVars      bool
	HasBody      bool
	Body         string
	BodyField    string
	BodyQueryName string
	BodyHTTPBody bool
	BodyMessage  bool
	ResponseBody string
	ResponseBodyField string
	ResponseBodyHTTPBody bool
	ReplyHTTPBody        bool
	ClientStreaming      bool
	ServerStreaming      bool
	URIParams            []pathutil.ParamBinding
}

func (s *serviceDesc) hasHTTPBody() bool {
	for _, m := range s.Methods {
		if m.BodyHTTPBody || m.ResponseBodyHTTPBody {
			return true
		}
	}
	return false
}

func (s *serviceDesc) execute() string {
	s.MethodSets = make(map[string]*methodDesc)
	for _, m := range s.Methods {
		s.MethodSets[m.Name] = m
	}
	sort.SliceStable(s.Methods, func(i, j int) bool {
		pi := strings.Contains(s.Methods[i].FiberPath, ":")
		pj := strings.Contains(s.Methods[j].FiberPath, ":")
		if pi != pj {
			return !pi // 静态路由优先于 :param，避免 /helloworld/chat 被 /helloworld/:name 截获
		}
		return len(s.Methods[i].FiberPath) > len(s.Methods[j].FiberPath)
	})
	buf := new(bytes.Buffer)
	tmpl, err := template.New("fiber").Funcs(template.FuncMap{
		"lower": strings.ToLower,
		"eq":    func(a, b string) bool { return a == b },
		"isNestedURI": func(fieldPath string) bool { return strings.Contains(fieldPath, ".") },
	}).Parse(strings.TrimSpace(fiberTemplate))
	if err != nil {
		panic(err)
	}
	if err := tmpl.Execute(buf, s); err != nil {
		panic(err)
	}
	return strings.Trim(buf.String(), "\r\n")
}
