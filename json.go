// Package fxmdjson provides independent native FastJSON views of FXMacroData JSON.
// It performs no network I/O and accepts no credentials or client configuration.
package fxmdjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/valyala/fastjson"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	MaxResponseBytes = 4 << 20
	MaxViewBytes     = 8 << 20
	MaxJSONDepth     = 64
	MaxJSONValues    = 50000
)

// Document owns the complete original response and an immutable parsed view.
// Native arithmetic results never overwrite observations or metadata.
type Document struct {
	raw   json.RawMessage
	value any
}

// Parse serializes a customer-owned SDK model. ParseJSON preserves original HTTP
// bytes and unknown fields that a typed model might otherwise omit.
func Parse(data any) (*Document, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, errors.New("response is not JSON-serializable")
	}
	return ParseJSON(raw)
}
func ParseJSON(raw []byte) (*Document, error) {
	if len(raw) == 0 || len(raw) > MaxResponseBytes || !utf8.Valid(raw) {
		return nil, errors.New("response size or encoding is invalid")
	}
	owned := append(json.RawMessage(nil), raw...)
	decoder := json.NewDecoder(bytes.NewReader(owned))
	decoder.UseNumber()
	remaining := MaxJSONValues
	value, err := decode(decoder, 0, &remaining)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, errors.New("exactly one JSON value is required")
	}
	return &Document{raw: owned, value: value}, nil
}

// Raw returns an independent copy of the unmodified full response.
func (d *Document) Raw() json.RawMessage {
	if d == nil {
		return nil
	}
	return append(json.RawMessage(nil), d.raw...)
}

// Exists distinguishes missing fields from explicit JSON null.
func (d *Document) Exists(pointer string) (bool, error) {
	if d == nil {
		return false, errors.New("a document is required")
	}
	_, found, err := lookup(d.value, pointer)
	return found, err
}
func (d *Document) selected(pointer string) (any, error) {
	if d == nil {
		return nil, errors.New("a document is required")
	}
	value, found, err := lookup(d.value, pointer)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.New("JSON field is unavailable")
	}
	return value, nil
}

// JSONAt returns a compact subtree view, retaining numeric spelling. Raw retains
// original whitespace, escaping, member order and the complete surrounding data.
func (d *Document) JSONAt(pointer string) (json.RawMessage, error) {
	value, err := d.selected(pointer)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(value)
	if err != nil || len(raw) > MaxViewBytes {
		return nil, errors.New("JSON view exceeds bounds or cannot be encoded")
	}
	return raw, nil
}

// NumberTextAt returns an original JSON number lexeme without float conversion.
func (d *Document) NumberTextAt(pointer string) (string, error) {
	value, err := d.selected(pointer)
	if err != nil {
		return "", err
	}
	number, ok := value.(json.Number)
	if !ok {
		return "", errors.New("JSON value is not a number")
	}
	return number.String(), nil
}

func lookup(root any, pointer string) (any, bool, error) {
	if len(pointer) > 4096 || !utf8.ValidString(pointer) || strings.Count(pointer, "/") > MaxJSONDepth+1 {
		return nil, false, errors.New("JSON Pointer exceeds supported bounds")
	}
	if pointer == "" {
		return root, true, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, false, errors.New("invalid JSON Pointer")
	}
	current := root
	for _, part := range strings.Split(pointer[1:], "/") {
		var name strings.Builder
		for i := 0; i < len(part); i++ {
			if part[i] != '~' {
				name.WriteByte(part[i])
				continue
			}
			i++
			if i >= len(part) {
				return nil, false, errors.New("invalid JSON Pointer escape")
			}
			switch part[i] {
			case '0':
				name.WriteByte('~')
			case '1':
				name.WriteByte('/')
			default:
				return nil, false, errors.New("invalid JSON Pointer escape")
			}
		}
		key := name.String()
		switch value := current.(type) {
		case map[string]any:
			var found bool
			current, found = value[key]
			if !found {
				return nil, false, nil
			}
		case []any:
			if key == "" || (len(key) > 1 && key[0] == '0') {
				return nil, false, errors.New("invalid JSON array index")
			}
			for _, c := range key {
				if c < '0' || c > '9' {
					return nil, false, errors.New("invalid JSON array index")
				}
			}
			index, err := strconv.ParseUint(key, 10, 64)
			if err != nil || index >= uint64(len(value)) {
				return nil, false, nil
			}
			current = value[int(index)]
		default:
			return nil, false, nil
		}
	}
	return current, true, nil
}

func decode(dec *json.Decoder, depth int, remaining *int) (any, error) {
	if depth > MaxJSONDepth {
		return nil, errors.New("nesting limit exceeded")
	}
	*remaining--
	if *remaining < 0 {
		return nil, errors.New("value limit exceeded")
	}
	token, err := dec.Token()
	if err != nil {
		return nil, errors.New("malformed JSON")
	}
	switch value := token.(type) {
	case json.Delim:
		switch value {
		case '{':
			object := map[string]any{}
			for dec.More() {
				key, err := dec.Token()
				if err != nil {
					return nil, errors.New("malformed object")
				}
				name, ok := key.(string)
				if !ok {
					return nil, errors.New("non-string object key")
				}
				if _, exists := object[name]; exists {
					return nil, errors.New("duplicate object key")
				}
				item, err := decode(dec, depth+1, remaining)
				if err != nil {
					return nil, err
				}
				object[name] = item
			}
			close, err := dec.Token()
			if err != nil || close != json.Delim('}') {
				return nil, errors.New("malformed object")
			}
			return object, nil
		case '[':
			array := []any{}
			for dec.More() {
				item, err := decode(dec, depth+1, remaining)
				if err != nil {
					return nil, err
				}
				array = append(array, item)
			}
			close, err := dec.Token()
			if err != nil || close != json.Delim(']') {
				return nil, errors.New("malformed array")
			}
			return array, nil
		}
		return nil, errors.New("unexpected delimiter")
	case json.Number:
		return value, nil
	case string, bool, nil:
		return token, nil
	}
	return nil, errors.New("unsupported JSON token")
}

// Native returns an independent native view of the complete response.
// No parser instance is reused, so later calls cannot invalidate earlier views.
func (d *Document) Native() (*fastjson.Value, error) { return d.NativeAt("") }

// NativeAt selects a complete JSON value by RFC 6901 pointer and constructs an
// independently owned native tree. Native edits never affect Document or Raw.
// Numeric lexemes are retained; native numeric conversions are the caller's choice.
func (d *Document) NativeAt(pointer string) (*fastjson.Value, error) {
	raw, err := d.JSONAt(pointer)
	if err != nil {
		return nil, err
	}
	value, err := fastjson.Parse(string(raw))
	if err != nil {
		return nil, errors.New("selected JSON value cannot be represented by the native parser")
	}
	return ownedNative(value), nil
}

// Detach every node, including primitive nodes that a native parser may intern.
// Container backing belongs to this fresh parse and is not exposed beforehand.
func ownedNative(value *fastjson.Value) *fastjson.Value {
	copy := *value
	switch copy.Type() {
	case fastjson.TypeObject:
		object, _ := copy.Object()
		type member struct {
			key   string
			value *fastjson.Value
		}
		members := make([]member, 0)
		object.Visit(func(key []byte, v *fastjson.Value) { members = append(members, member{string(key), v}) })
		for _, m := range members {
			copy.Set(m.key, ownedNative(m.value))
		}
	case fastjson.TypeArray:
		values, _ := copy.Array()
		for i, v := range values {
			values[i] = ownedNative(v)
		}
	}
	return &copy
}
