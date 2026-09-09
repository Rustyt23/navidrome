package aitools

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Schema is the JSON Schema subset that every configured provider accepts.
//
// It is deliberately small. Gemini's function declarations take an OpenAPI 3.0
// subset, Bedrock's toolSpec takes draft-2020 JSON Schema, and the prompted
// fallback takes whatever we print at it. The intersection is: type,
// description, format, enum, items, properties, required, nullable. Anything
// outside that risks a provider rejecting the declaration outright, so
// additionalProperties in particular is enforced locally (see strict) rather
// than emitted on the wire.
type Schema struct {
	Type        string             `json:"type,omitempty"`
	Description string             `json:"description,omitempty"`
	Format      string             `json:"format,omitempty"`
	Enum        []string           `json:"enum,omitempty"`
	Items       *Schema            `json:"items,omitempty"`
	Properties  map[string]*Schema `json:"properties,omitempty"`
	Required    []string           `json:"required,omitempty"`
	Nullable    bool               `json:"nullable,omitempty"`

	// strict rejects properties the schema does not declare. It is not part of
	// the wire format because no provider agrees on how to express it.
	//
	// Rejecting is worth the strictness because the alternative failure is
	// silent: encoding/json drops unknown keys, so a model that guesses
	// "play_count_min" instead of "playCountMin" gets results with no filter
	// applied and reports back that it filtered. A returned error is
	// recoverable on the next turn; a wrong answer stated confidently is not.
	strict bool
}

const (
	TypeObject  = "object"
	TypeArray   = "array"
	TypeString  = "string"
	TypeInteger = "integer"
	TypeNumber  = "number"
	TypeBoolean = "boolean"
)

var (
	timeType       = reflect.TypeFor[time.Time]()
	rawMessageType = reflect.TypeFor[json.RawMessage]()
)

// SchemaOf derives a schema from the Go type a tool handler unmarshals into.
//
// Deriving beats hand-writing because the two cannot then disagree: a filter
// added to rag.SearchFilters for the UI becomes callable by the model in the
// same commit, and a renamed field cannot leave a stale schema behind
// advertising an argument that is now silently discarded.
//
// Field naming follows the json tag. A field is required unless it is a
// pointer or carries omitempty, which matches how the same struct behaves when
// the HTTP handlers decode it.
func SchemaOf[T any]() Schema {
	t := reflect.TypeFor[T]()
	return *schemaForType(t, map[reflect.Type]bool{})
}

// fieldOptions is the part of a struct field's tags the schema cares about.
type fieldOptions struct {
	description string
	enum        []string
	format      string
}

// parseSchemaTag reads the optional `jsonschema` tag. Go struct tags give us
// no other place to put the field descriptions that tool-calling accuracy
// depends on most.
//
// Entries are separated by ';' rather than ',' so a description can contain
// commas, which they nearly always do:
//
//	jsonschema:"description=Only songs above this BPM;format=float"
//	jsonschema:"enum=add|remove|move|replace"
func parseSchemaTag(tag string) fieldOptions {
	var opts fieldOptions
	for _, part := range strings.Split(tag, ";") {
		key, value, found := strings.Cut(strings.TrimSpace(part), "=")
		if !found {
			continue
		}
		switch strings.TrimSpace(key) {
		case "description":
			opts.description = value
		case "format":
			opts.format = value
		case "enum":
			for _, v := range strings.Split(value, "|") {
				if v = strings.TrimSpace(v); v != "" {
					opts.enum = append(opts.enum, v)
				}
			}
		}
	}
	return opts
}

// schemaForType maps one Go type. seen holds the struct types on the current
// recursion path so a self-referential type terminates; it is unwound on the
// way out so two sibling fields of the same type both get full schemas.
func schemaForType(t reflect.Type, seen map[reflect.Type]bool) *Schema {
	nullable := false
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
		nullable = true
	}

	s := &Schema{Nullable: nullable}

	// Checked before the Kind switch: json.RawMessage is a []byte and
	// time.Time is a struct, and neither should be described structurally.
	switch t {
	case rawMessageType:
		return s // no constraint: any JSON value
	case timeType:
		s.Type, s.Format = TypeString, "date-time"
		return s
	}

	switch t.Kind() {
	case reflect.Bool:
		s.Type = TypeBoolean
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		s.Type = TypeInteger
	case reflect.Float32, reflect.Float64:
		s.Type = TypeNumber
	case reflect.String:
		s.Type = TypeString
	case reflect.Slice, reflect.Array:
		// encoding/json renders []byte as a base64 string, so the schema has
		// to say string or the model will send an array of numbers.
		if t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8 {
			s.Type = TypeString
			return s
		}
		s.Type = TypeArray
		s.Items = schemaForType(t.Elem(), seen)
	case reflect.Map:
		// Only the value type is describable; JSON object keys are strings.
		s.Type = TypeObject
	case reflect.Struct:
		if seen[t] {
			// A cycle. Describe it as an open object rather than recursing:
			// the alternative is an infinite schema, and no provider accepts
			// $ref consistently across all three dialects.
			s.Type = TypeObject
			return s
		}
		seen[t] = true
		defer delete(seen, t)

		s.Type = TypeObject
		s.strict = true
		s.Properties = map[string]*Schema{}
		collectFields(t, s, seen)
	case reflect.Interface:
		// any: no constraint.
	default:
		// Channels, funcs and the like cannot appear in a decoded argument
		// struct. Leaving the schema empty keeps SchemaOf total.
	}
	return s
}

// collectFields walks a struct's fields into s, flattening embedded structs
// the way encoding/json does.
func collectFields(t reflect.Type, s *Schema, seen map[reflect.Type]bool) {
	for i := range t.NumField() {
		field := t.Field(i)
		if !field.IsExported() && !field.Anonymous {
			continue
		}

		jsonTag := field.Tag.Get("json")
		name, tagOpts, _ := strings.Cut(jsonTag, ",")
		if name == "-" && tagOpts == "" {
			continue
		}

		// An embedded struct with no explicit json name is inlined by
		// encoding/json, so its fields belong to the parent object.
		if field.Anonymous && name == "" {
			embedded := field.Type
			for embedded.Kind() == reflect.Pointer {
				embedded = embedded.Elem()
			}
			if embedded.Kind() == reflect.Struct {
				collectFields(embedded, s, seen)
				continue
			}
		}
		if !field.IsExported() {
			continue
		}
		if name == "" {
			name = field.Name
		}

		prop := schemaForType(field.Type, seen)
		opts := parseSchemaTag(field.Tag.Get("jsonschema"))
		if opts.description != "" {
			prop.Description = opts.description
		}
		if opts.format != "" {
			prop.Format = opts.format
		}
		if len(opts.enum) > 0 {
			prop.Enum = opts.enum
		}
		s.Properties[name] = prop

		// Required mirrors the decode contract the HTTP handlers already work
		// to: omitempty or a pointer means the caller may leave it out.
		optional := field.Type.Kind() == reflect.Pointer ||
			strings.Contains(","+tagOpts+",", ",omitempty,")
		if !optional {
			s.Required = append(s.Required, name)
		}
	}
}

// Validate checks decoded arguments against the schema.
//
// This is not a general JSON Schema validator and does not try to be. It
// covers the failures a language model actually produces - a missing required
// argument, a number sent as a string, a guessed field name, a value outside
// an enum - and it reports them with a path so the error can be handed back to
// the model as something it can act on rather than as a stack trace.
func (s Schema) Validate(raw json.RawMessage) error {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		// Providers omit the arguments object entirely for a tool that needs
		// none. Treated as {} so a no-argument tool validates, while a tool
		// with required arguments still reports them as missing.
		trimmed = "{}"
	}

	decoder := json.NewDecoder(strings.NewReader(trimmed))
	decoder.UseNumber() // keeps 3 distinguishable from 3.5 for integer fields

	var value any
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("arguments are not valid JSON: %w", err)
	}
	return s.validateValue(value, "")
}

func (s Schema) validateValue(value any, path string) error {
	if value == nil {
		if s.Nullable || s.Type == "" {
			return nil
		}
		return fmt.Errorf("%s must not be null", pathOrValue(path))
	}

	switch s.Type {
	case "": // unconstrained
		return nil

	case TypeString:
		str, ok := value.(string)
		if !ok {
			return typeError(path, TypeString, value)
		}
		if len(s.Enum) > 0 && !slices.Contains(s.Enum, str) {
			return fmt.Errorf("%s must be one of %s, got %q",
				pathOrValue(path), strings.Join(s.Enum, ", "), str)
		}

	case TypeBoolean:
		if _, ok := value.(bool); !ok {
			return typeError(path, TypeBoolean, value)
		}

	case TypeNumber:
		num, ok := value.(json.Number)
		if !ok {
			return typeError(path, TypeNumber, value)
		}
		if _, err := num.Float64(); err != nil {
			return typeError(path, TypeNumber, value)
		}

	case TypeInteger:
		num, ok := value.(json.Number)
		if !ok {
			return typeError(path, TypeInteger, value)
		}
		if _, err := strconv.ParseInt(num.String(), 10, 64); err != nil {
			return typeError(path, TypeInteger, value)
		}

	case TypeArray:
		items, ok := value.([]any)
		if !ok {
			return typeError(path, TypeArray, value)
		}
		if s.Items == nil {
			return nil
		}
		for i, item := range items {
			if err := s.Items.validateValue(item, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}

	case TypeObject:
		object, ok := value.(map[string]any)
		if !ok {
			return typeError(path, TypeObject, value)
		}
		for _, name := range s.Required {
			if v, present := object[name]; !present || v == nil {
				return fmt.Errorf("%s is required", joinPath(path, name))
			}
		}
		for name, item := range object {
			prop, declared := s.Properties[name]
			if !declared {
				if s.strict && len(s.Properties) > 0 {
					return fmt.Errorf("%s is not a valid argument (expected one of: %s)",
						joinPath(path, name), strings.Join(sortedKeys(s.Properties), ", "))
				}
				continue
			}
			if err := prop.validateValue(item, joinPath(path, name)); err != nil {
				return err
			}
		}
	}
	return nil
}

func typeError(path, want string, got any) error {
	return fmt.Errorf("%s must be %s, got %s", pathOrValue(path), want, jsonTypeName(got))
}

func jsonTypeName(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case json.Number:
		if _, err := strconv.ParseInt(t.String(), 10, 64); err == nil {
			return "integer"
		}
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return reflect.TypeOf(v).String()
	}
}

func joinPath(path, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}

// pathOrValue names the offending location. The root has no path, and "value"
// reads better there than an empty string.
func pathOrValue(path string) string {
	if path == "" {
		return "value"
	}
	return path
}

func sortedKeys(m map[string]*Schema) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Sorted so the error message a model reads back is stable between calls.
	slices.Sort(keys)
	return keys
}
