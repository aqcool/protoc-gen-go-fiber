// Package binding 是生成代码依赖的薄运行时，负责 protojson 编解码、路径构建与流式帧协议。
// 生成 handler 直接调用 Fiber c.Bind()，binding 仅处理无法由模板表达的横切逻辑。
package binding

import (
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var (
	installed sync.Map
	marshaler = protojson.MarshalOptions{UseProtoNames: true}
	unmarshaler = protojson.UnmarshalOptions{DiscardUnknown: true}
)

type protoJSONBinding struct{}

func (b *protoJSONBinding) Name() string { return "protojson" }

func (b *protoJSONBinding) MIMETypes() []string {
	return []string{"application/json", "application/protojson"}
}

func (b *protoJSONBinding) Parse(c fiber.Ctx, out any) error {
	msg, ok := out.(proto.Message)
	if !ok {
		if ptr, ok := out.(*proto.Message); ok && ptr != nil {
			msg = *ptr
		} else {
			return fmt.Errorf("binding: target must implement proto.Message")
		}
	}
	if isHTTPBodyMessage(msg.ProtoReflect().Descriptor()) {
		return setHTTPBody(msg, c.Get("Content-Type"), c.Body())
	}
	return unmarshaler.Unmarshal(c.Body(), msg)
}

var httpBodyFullName = protoreflect.FullName("google.api.HttpBody")

func isHTTPBodyMessage(md protoreflect.MessageDescriptor) bool {
	return md != nil && md.FullName() == httpBodyFullName
}

func setHTTPBody(msg proto.Message, contentType string, data []byte) error {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	ref := msg.ProtoReflect()
	if ct := ref.Descriptor().Fields().ByName("content_type"); ct != nil {
		ref.Set(ct, protoreflect.ValueOfString(contentType))
	}
	if dataFD := ref.Descriptor().Fields().ByName("data"); dataFD != nil {
		ref.Set(dataFD, protoreflect.ValueOfBytes(append([]byte(nil), data...)))
	}
	return nil
}

// BindHTTPBody assigns the raw HTTP body to a google.api.HttpBody request field.
func BindHTTPBody(c fiber.Ctx, msg proto.Message, fieldName string) error {
	fd := msg.ProtoReflect().Descriptor().Fields().ByName(protoreflect.Name(fieldName))
	if fd == nil {
		return fmt.Errorf("binding: unknown field %q", fieldName)
	}
	if !isHTTPBodyMessage(fd.Message()) {
		return fmt.Errorf("binding: field %q is not google.api.HttpBody", fieldName)
	}
	contentType := c.Get("Content-Type")
	if contentType == "" {
		contentType = string(c.Request().Header.ContentType())
	}
	body := msg.ProtoReflect().Mutable(fd).Message().Interface()
	return setHTTPBody(body, contentType, c.Body())
}
// Ensure 在首次调用时向 Fiber App 注册 protojson CustomBinder（sync.Once，每个 App 一次）。
// 生成 handler 首行调用，无需用户手动 bootstrap；兼容 fiber.App 与 fiber.Group。
func Ensure(c fiber.Ctx) {
	app := c.App()
	once, _ := installed.LoadOrStore(app, &sync.Once{})
	once.(*sync.Once).Do(func() {
		app.RegisterCustomBinder(&protoJSONBinding{})
	})
}

// WriteOption configures response encoding.
type WriteOption func(*writeOptions)

type writeOptions struct {
	fieldPath string
	httpBody  bool
}

// WithField encodes only the given top-level response field (response_body).
func WithField(fieldPath string) WriteOption {
	return func(o *writeOptions) { o.fieldPath = strings.TrimPrefix(fieldPath, ".") }
}

// WithHTTPBody marks the payload as google.api.HttpBody bytes.
func WithHTTPBody() WriteOption {
	return func(o *writeOptions) { o.httpBody = true }
}

// Write encodes a proto message as protojson HTTP response.
func Write(c fiber.Ctx, status int, msg proto.Message, opts ...WriteOption) error {
	options := writeOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	if options.httpBody {
		if options.fieldPath != "" {
			sub, err := extractMessage(msg, options.fieldPath)
			if err != nil {
				return err
			}
			if hb, ok := sub.(interface{ GetContentType() string }); ok && hb.GetContentType() != "" {
				c.Type(hb.GetContentType())
			}
			msg = sub
		} else if body, ok := msg.ProtoReflect().Interface().(interface{ GetContentType() string }); ok {
			c.Type(body.GetContentType())
		}
		data, err := extractBytes(msg, "")
		if err != nil {
			return err
		}
		return c.Status(status).Send(data)
	}
	if options.fieldPath != "" {
		msg, err := extractMessage(msg, options.fieldPath)
		if err != nil {
			return err
		}
		data, err := marshaler.Marshal(msg)
		if err != nil {
			return err
		}
		c.Type("application/json")
		return c.Status(status).Send(data)
	}
	data, err := marshaler.Marshal(msg)
	if err != nil {
		return err
	}
	c.Type("application/json")
	return c.Status(status).Send(data)
}

func extractBytes(msg proto.Message, fieldPath string) ([]byte, error) {
	target := msg
	if fieldPath != "" {
		sub, err := extractMessage(msg, fieldPath)
		if err != nil {
			return nil, err
		}
		target = sub
	}
	fd := target.ProtoReflect().Descriptor().Fields().ByName("data")
	if fd != nil {
		return target.ProtoReflect().Get(fd).Bytes(), nil
	}
	return nil, fmt.Errorf("binding: message has no data field")
}

func extractMessage(msg proto.Message, fieldPath string) (proto.Message, error) {
	val, err := fieldValue(msg, fieldPath)
	if err != nil {
		return nil, err
	}
	if val.Message().IsValid() {
		return val.Message().Interface(), nil
	}
	return nil, fmt.Errorf("binding: field %q is not a message", fieldPath)
}

func fieldValue(msg proto.Message, fieldPath string) (protoreflect.Value, error) {
	parts := strings.Split(fieldPath, ".")
	current := msg.ProtoReflect()
	for i, part := range parts {
		fd := current.Descriptor().Fields().ByName(protoreflect.Name(part))
		if fd == nil {
			return protoreflect.Value{}, fmt.Errorf("binding: unknown field %q", part)
		}
		val := current.Get(fd)
		if i == len(parts)-1 {
			return val, nil
		}
		if fd.Kind() != protoreflect.MessageKind {
			return protoreflect.Value{}, fmt.Errorf("binding: field %q is not a message", part)
		}
		current = val.Message()
	}
	return protoreflect.Value{}, fmt.Errorf("binding: empty field path")
}

// SetURIParam assigns a path parameter value to a dotted proto field path.
func SetURIParam(msg proto.Message, fieldPath, value string) error {
	parts := strings.Split(fieldPath, ".")
	current := msg.ProtoReflect()
	for i, part := range parts {
		fd := current.Descriptor().Fields().ByName(protoreflect.Name(part))
		if fd == nil {
			return fmt.Errorf("binding: unknown field %q", part)
		}
		if i == len(parts)-1 {
			return setURIValue(fd, current, value)
		}
		if fd.Kind() != protoreflect.MessageKind {
			return fmt.Errorf("binding: field %q is not a message", part)
		}
		current = current.Mutable(fd).Message()
	}
	return nil
}

func setURIValue(fd protoreflect.FieldDescriptor, msg protoreflect.Message, value string) error {
	switch fd.Kind() {
	case protoreflect.StringKind:
		msg.Set(fd, protoreflect.ValueOfString(value))
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		var v int32
		if _, err := fmt.Sscan(value, &v); err != nil {
			return err
		}
		msg.Set(fd, protoreflect.ValueOfInt32(v))
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		var v int64
		if _, err := fmt.Sscan(value, &v); err != nil {
			return err
		}
		msg.Set(fd, protoreflect.ValueOfInt64(v))
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		var v uint32
		if _, err := fmt.Sscan(value, &v); err != nil {
			return err
		}
		msg.Set(fd, protoreflect.ValueOfUint32(v))
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		var v uint64
		if _, err := fmt.Sscan(value, &v); err != nil {
			return err
		}
		msg.Set(fd, protoreflect.ValueOfUint64(v))
	case protoreflect.BoolKind:
		var v bool
		if _, err := fmt.Sscan(value, &v); err != nil {
			return err
		}
		msg.Set(fd, protoreflect.ValueOfBool(v))
	default:
		return fmt.Errorf("binding: unsupported URI field kind %s", fd.Kind())
	}
	return nil
}

var pathTemplateParamRE = regexp.MustCompile(`{([.\w]+)(=[^{}]*)?}`)

// PathOption configures client path construction.
type PathOption func(*pathOptions)

type pathOptions struct {
	queryParams bool
	omitFields  []string
}

// WithQueryParams appends unbound fields as query parameters.
func WithQueryParams() PathOption {
	return func(o *pathOptions) { o.queryParams = true }
}

// WithOmitFields excludes fields from query parameters.
func WithOmitFields(fields ...string) PathOption {
	return func(o *pathOptions) { o.omitFields = append(o.omitFields, fields...) }
}

// Path builds an HTTP request path from a Google API path template and request message.
func Path(pathTemplate string, msg proto.Message, opts ...PathOption) string {
	if msg == nil || !msg.ProtoReflect().IsValid() {
		return pathTemplate
	}
	options := pathOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	queryParams, _ := encodeValues(msg)
	pathParams := make(map[string]struct{})
	path := pathTemplate
	if strings.ContainsRune(pathTemplate, '{') {
		path = pathTemplateParamRE.ReplaceAllStringFunc(pathTemplate, func(in string) string {
			matches := pathTemplateParamRE.FindStringSubmatch(in)
			key := matches[1]
			pathParams[key] = struct{}{}
			return queryParams.Get(key)
		})
	}
	if !options.queryParams {
		return path
	}
	if len(queryParams) > 0 {
		for key := range pathParams {
			delete(queryParams, key)
		}
		omitQueryParams(queryParams, options.omitFields)
		if query := queryParams.Encode(); query != "" {
			path += "?" + query
		}
	}
	return path
}

func omitQueryParams(values map[string][]string, fields []string) {
	for _, field := range fields {
		if field == "" {
			continue
		}
		delete(values, field)
		prefix := field + "."
		for key := range values {
			if strings.HasPrefix(key, prefix) {
				delete(values, key)
			}
		}
	}
}

func encodeValues(msg proto.Message) (url.Values, error) {
	u := make(url.Values)
	err := encodeByField(u, "", msg.ProtoReflect())
	return u, err
}

func encodeByField(u url.Values, path string, m protoreflect.Message) error {
	var finalErr error
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		key := fd.JSONName()
		if key == "" {
			key = string(fd.Name())
		}
		newPath := key
		if path != "" {
			newPath = path + "." + key
		}
		if of := fd.ContainingOneof(); of != nil {
			if f := m.WhichOneof(of); f != nil && f != fd {
				return true
			}
		}
		switch {
		case fd.IsList():
			list := v.List()
			for i := 0; i < list.Len(); i++ {
				if s, err := scalarString(fd, list.Get(i)); err != nil {
					finalErr = err
				} else {
					u.Add(newPath, s)
				}
			}
		case fd.IsMap():
			v.Map().Range(func(k protoreflect.MapKey, val protoreflect.Value) bool {
				if s, err := scalarString(fd.MapValue(), val); err != nil {
					finalErr = err
				} else {
					u.Set(newPath+"["+k.String()+"]", s)
				}
				return true
			})
		case fd.Kind() == protoreflect.MessageKind:
			if err := encodeByField(u, newPath, v.Message()); err != nil {
				finalErr = err
			}
		default:
			if s, err := scalarString(fd, v); err != nil {
				finalErr = err
			} else {
				u.Set(newPath, s)
			}
		}
		return true
	})
	return finalErr
}

func scalarString(fd protoreflect.FieldDescriptor, v protoreflect.Value) (string, error) {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		return fmt.Sprintf("%v", v.Bool()), nil
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return fmt.Sprintf("%d", v.Int()), nil
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return fmt.Sprintf("%d", v.Int()), nil
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return fmt.Sprintf("%d", v.Uint()), nil
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return fmt.Sprintf("%d", v.Uint()), nil
	case protoreflect.FloatKind:
		return fmt.Sprintf("%v", v.Float()), nil
	case protoreflect.DoubleKind:
		return fmt.Sprintf("%v", v.Float()), nil
	case protoreflect.StringKind:
		return v.String(), nil
	case protoreflect.BytesKind:
		return string(v.Bytes()), nil
	case protoreflect.EnumKind:
		return fmt.Sprintf("%d", v.Enum()), nil
	default:
		return "", fmt.Errorf("binding: unsupported field kind %s", fd.Kind())
	}
}

// BodyContentType returns Content-Type for google.api.HttpBody.
func BodyContentType(msg proto.Message) string {
	if msg == nil {
		return "application/protojson"
	}
	rv := reflect.ValueOf(msg)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if m, ok := rv.Interface().(interface{ GetContentType() string }); ok {
		if ct := m.GetContentType(); ct != "" {
			return ct
		}
	}
	return "application/octet-stream"
}
