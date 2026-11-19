package main

// Spec-driven conformance checks for the CIP-0056 registry API. The schemas come from the
// vendored openapi/*.yaml files at test time. The YAML reader below accepts only the subset these
// files use and fails on any other construct, so a spec change it cannot read stops the test
// instead of weakening it. The validator fails on any schema keyword outside its supported set.
//
// The reader and validator recurse over the document tree. Both stop at maxOpenAPIDepth.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const maxOpenAPIDepth = 64

// Vendored spec files, by API.
const (
	specTokenMetadataV1         = "token-metadata-v1.yaml"
	specTransferInstructionV1   = "transfer-instruction-v1.yaml"
	specAllocationInstructionV1 = "allocation-instruction-v1.yaml"
	specAllocationV1            = "allocation-v1.yaml"
)

func vendoredSpecFiles() []string {
	return []string{specTokenMetadataV1, specTransferInstructionV1, specAllocationInstructionV1, specAllocationV1}
}

// ---------------------------------------------------------------------------
// YAML subset reader
// ---------------------------------------------------------------------------

type yamlSourceLine struct {
	number int
	indent int
	text   string
	blank  bool
}

type yamlSubsetReader struct {
	lines    []yamlSourceLine
	position int
}

// parseYAMLSubset reads block mappings, block sequences, quoted and plain scalars, and literal
// or folded block scalars. Flow collections other than [] and {}, anchors, aliases, tags,
// multi-line plain scalars, and inline comments are errors.
func parseYAMLSubset(source string) (any, error) {
	reader := &yamlSubsetReader{}
	for index, raw := range strings.Split(source, "\n") {
		trimmedRight := strings.TrimRight(raw, " \r")
		content := strings.TrimLeft(trimmedRight, " ")
		if strings.HasPrefix(content, "\t") {
			return nil, fmt.Errorf("line %d: tab indentation", index+1)
		}
		line := yamlSourceLine{number: index + 1, indent: len(trimmedRight) - len(content), text: content}
		line.blank = content == "" || strings.HasPrefix(content, "#")
		reader.lines = append(reader.lines, line)
	}
	reader.skipBlank()
	if reader.position >= len(reader.lines) {
		return nil, fmt.Errorf("empty document")
	}
	root, err := reader.parseBlock(reader.lines[reader.position].indent, 0)
	if err != nil {
		return nil, err
	}
	reader.skipBlank()
	if reader.position != len(reader.lines) {
		return nil, fmt.Errorf("line %d: unexpected content after the document", reader.lines[reader.position].number)
	}
	return root, nil
}

func (reader *yamlSubsetReader) skipBlank() {
	for reader.position < len(reader.lines) && reader.lines[reader.position].blank {
		reader.position++
	}
}

func isYAMLSequenceItem(text string) bool {
	return text == "-" || strings.HasPrefix(text, "- ")
}

func (reader *yamlSubsetReader) parseBlock(indent, depth int) (any, error) {
	if depth > maxOpenAPIDepth {
		return nil, fmt.Errorf("nesting deeper than %d", maxOpenAPIDepth)
	}
	line := reader.lines[reader.position]
	if line.indent != indent {
		return nil, fmt.Errorf("line %d: indent %d, expected %d", line.number, line.indent, indent)
	}
	if isYAMLSequenceItem(line.text) {
		return reader.parseSequence(indent, depth)
	}
	return reader.parseMapping(indent, depth)
}

// parseNestedValue parses the block under a "key:" or "-" line whose own indent is parentIndent.
func (reader *yamlSubsetReader) parseNestedValue(parentIndent int, allowSameIndentSequence bool, depth int) (any, error) {
	reader.skipBlank()
	if reader.position >= len(reader.lines) {
		return nil, nil
	}
	next := reader.lines[reader.position]
	if next.indent > parentIndent {
		return reader.parseBlock(next.indent, depth+1)
	}
	if allowSameIndentSequence && next.indent == parentIndent && isYAMLSequenceItem(next.text) {
		return reader.parseSequence(parentIndent, depth+1)
	}
	return nil, nil
}

func (reader *yamlSubsetReader) parseSequence(indent, depth int) (any, error) {
	items := make([]any, 0)
	for {
		reader.skipBlank()
		if reader.position >= len(reader.lines) {
			return items, nil
		}
		line := reader.lines[reader.position]
		if line.indent < indent || (line.indent == indent && !isYAMLSequenceItem(line.text)) {
			return items, nil
		}
		if line.indent > indent {
			return nil, fmt.Errorf("line %d: unexpected indentation in a sequence", line.number)
		}
		rest := strings.TrimLeft(strings.TrimPrefix(line.text, "-"), " ")
		if rest == "" {
			reader.position++
			item, err := reader.parseNestedValue(indent, false, depth)
			if err != nil {
				return nil, err
			}
			items = append(items, item)
			continue
		}
		if _, _, isEntry, err := splitYAMLMappingEntry(rest, line.number); err != nil {
			return nil, err
		} else if isEntry {
			itemIndent := indent + len(line.text) - len(rest)
			reader.lines[reader.position] = yamlSourceLine{number: line.number, indent: itemIndent, text: rest}
			item, err := reader.parseMapping(itemIndent, depth+1)
			if err != nil {
				return nil, err
			}
			items = append(items, item)
			continue
		}
		scalar, err := parseYAMLScalar(rest, line.number)
		if err != nil {
			return nil, err
		}
		reader.position++
		if err := reader.checkNoContinuation(indent, line.number); err != nil {
			return nil, err
		}
		items = append(items, scalar)
	}
}

func (reader *yamlSubsetReader) parseMapping(indent, depth int) (any, error) {
	mapping := make(map[string]any)
	for {
		reader.skipBlank()
		if reader.position >= len(reader.lines) {
			return mapping, nil
		}
		line := reader.lines[reader.position]
		if line.indent < indent {
			return mapping, nil
		}
		if line.indent > indent {
			return nil, fmt.Errorf("line %d: unexpected indentation in a mapping", line.number)
		}
		if isYAMLSequenceItem(line.text) {
			return nil, fmt.Errorf("line %d: sequence item where a mapping key is expected", line.number)
		}
		key, rest, isEntry, err := splitYAMLMappingEntry(line.text, line.number)
		if err != nil {
			return nil, err
		}
		if !isEntry {
			return nil, fmt.Errorf("line %d: expected \"key: value\", got %q", line.number, line.text)
		}
		if _, duplicate := mapping[key]; duplicate {
			return nil, fmt.Errorf("line %d: duplicate key %q", line.number, key)
		}
		reader.position++
		switch rest {
		case "":
			value, err := reader.parseNestedValue(indent, true, depth)
			if err != nil {
				return nil, err
			}
			mapping[key] = value
		case "|", "|-", "|+", ">", ">-", ">+":
			mapping[key] = reader.readBlockScalar(indent)
		default:
			scalar, err := parseYAMLScalar(rest, line.number)
			if err != nil {
				return nil, err
			}
			if err := reader.checkNoContinuation(indent, line.number); err != nil {
				return nil, err
			}
			mapping[key] = scalar
		}
	}
}

// readBlockScalar joins the lines deeper than indent. Line folding is irrelevant to validation.
func (reader *yamlSubsetReader) readBlockScalar(indent int) string {
	parts := make([]string, 0)
	for reader.position < len(reader.lines) {
		line := reader.lines[reader.position]
		if line.text != "" && line.indent <= indent {
			break
		}
		parts = append(parts, line.text)
		reader.position++
	}
	return strings.TrimRight(strings.Join(parts, "\n"), "\n")
}

// checkNoContinuation rejects a multi-line plain scalar: the next content line must not be deeper.
func (reader *yamlSubsetReader) checkNoContinuation(indent, lineNumber int) error {
	reader.skipBlank()
	if reader.position < len(reader.lines) && reader.lines[reader.position].indent > indent {
		return fmt.Errorf("line %d: multi-line plain scalars are outside the supported subset", lineNumber)
	}
	return nil
}

// splitYAMLMappingEntry splits "key: rest" or "key:". A quoted key may contain ": ".
func splitYAMLMappingEntry(text string, lineNumber int) (key, rest string, isEntry bool, err error) {
	if strings.HasPrefix(text, "'") || strings.HasPrefix(text, `"`) {
		quote := text[:1]
		closing := strings.Index(text[1:], quote)
		if closing < 0 {
			return "", "", false, fmt.Errorf("line %d: unterminated quoted key", lineNumber)
		}
		after := text[closing+2:]
		if after != ":" && !strings.HasPrefix(after, ": ") {
			return "", "", false, nil
		}
		parsedKey, err := parseYAMLScalar(text[:closing+2], lineNumber)
		if err != nil {
			return "", "", false, err
		}
		return fmt.Sprint(parsedKey), strings.TrimSpace(strings.TrimPrefix(after, ":")), true, nil
	}
	if strings.HasSuffix(text, ":") && !strings.Contains(text, ": ") {
		return strings.TrimSuffix(text, ":"), "", true, nil
	}
	separator := strings.Index(text, ": ")
	if separator < 0 {
		return "", "", false, nil
	}
	return text[:separator], strings.TrimSpace(text[separator+2:]), true, nil
}

func parseYAMLScalar(text string, lineNumber int) (any, error) {
	switch {
	case strings.HasPrefix(text, "'"):
		if len(text) < 2 || !strings.HasSuffix(text, "'") {
			return nil, fmt.Errorf("line %d: unterminated single-quoted scalar", lineNumber)
		}
		return strings.ReplaceAll(text[1:len(text)-1], "''", "'"), nil
	case strings.HasPrefix(text, `"`):
		unquoted, err := strconv.Unquote(text)
		if err != nil {
			return nil, fmt.Errorf("line %d: double-quoted scalar %s: %w", lineNumber, text, err)
		}
		return unquoted, nil
	case text == "[]":
		return []any{}, nil
	case text == "{}":
		return map[string]any{}, nil
	case strings.ContainsAny(text[:1], "[{&*!|>%@`"):
		return nil, fmt.Errorf("line %d: scalar %q is outside the supported subset", lineNumber, text)
	case strings.Contains(text, " #"):
		return nil, fmt.Errorf("line %d: inline comments are outside the supported subset", lineNumber)
	case text == "true":
		return true, nil
	case text == "false":
		return false, nil
	case text == "null" || text == "~":
		return nil, nil
	}
	if integer, err := strconv.ParseInt(text, 10, 64); err == nil {
		return integer, nil
	}
	return text, nil
}

// ---------------------------------------------------------------------------
// OpenAPI 3.0 schema validator (the keyword subset the vendored specs use)
// ---------------------------------------------------------------------------

type openAPISpec struct {
	file string
	root map[string]any
}

var (
	openAPISpecCacheMutex sync.Mutex
	openAPISpecCache      = map[string]*openAPISpec{}
)

func loadOpenAPISpec(t *testing.T, file string) *openAPISpec {
	t.Helper()
	openAPISpecCacheMutex.Lock()
	defer openAPISpecCacheMutex.Unlock()
	if cached, ok := openAPISpecCache[file]; ok {
		return cached
	}
	raw, err := os.ReadFile(filepath.Join("openapi", file))
	if err != nil {
		t.Fatalf("read spec %s: %v", file, err)
	}
	document, err := parseYAMLSubset(string(raw))
	if err != nil {
		t.Fatalf("parse spec %s: %v", file, err)
	}
	root, ok := document.(map[string]any)
	if !ok {
		t.Fatalf("spec %s: root is %T, want a mapping", file, document)
	}
	if root["openapi"] != "3.0.0" {
		t.Fatalf("spec %s: openapi version %v, want 3.0.0", file, root["openapi"])
	}
	spec := &openAPISpec{file: file, root: root}
	openAPISpecCache[file] = spec
	return spec
}

func lookupMapping(value any, keys ...string) (map[string]any, bool) {
	current := value
	for _, key := range keys {
		mapping, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = mapping[key]
		if !ok {
			return nil, false
		}
	}
	mapping, ok := current.(map[string]any)
	return mapping, ok
}

// resolveLocalRef follows "#/a/b/c" inside the spec.
func (spec *openAPISpec) resolveLocalRef(ref string) (map[string]any, error) {
	if !strings.HasPrefix(ref, "#/") {
		return nil, fmt.Errorf("%s: non-local $ref %q", spec.file, ref)
	}
	target, ok := lookupMapping(spec.root, strings.Split(strings.TrimPrefix(ref, "#/"), "/")...)
	if !ok {
		return nil, fmt.Errorf("%s: unresolvable $ref %q", spec.file, ref)
	}
	return target, nil
}

// operations lists every (path, method) pair the spec declares.
func (spec *openAPISpec) operations() []string {
	paths, _ := lookupMapping(spec.root, "paths")
	out := make([]string, 0)
	for path, item := range paths {
		methods, _ := item.(map[string]any)
		for method := range methods {
			out = append(out, strings.ToUpper(method)+" "+path)
		}
	}
	sort.Strings(out)
	return out
}

// responseSchema returns the application/json schema of one declared response.
func (spec *openAPISpec) responseSchema(path, method string, status int) (map[string]any, error) {
	operation, ok := lookupMapping(spec.root, "paths", path, strings.ToLower(method))
	if !ok {
		return nil, fmt.Errorf("%s: no operation %s %s", spec.file, method, path)
	}
	response, declared := lookupMapping(operation, "responses", strconv.Itoa(status))
	if !declared {
		return nil, fmt.Errorf("%s: %s %s declares no %d response", spec.file, method, path, status)
	}
	if ref, isRef := response["$ref"].(string); isRef {
		resolved, err := spec.resolveLocalRef(ref)
		if err != nil {
			return nil, err
		}
		response = resolved
	}
	schema, ok := lookupMapping(response, "content", "application/json", "schema")
	if !ok {
		return nil, fmt.Errorf("%s: %s %s %d has no application/json schema", spec.file, method, path, status)
	}
	return schema, nil
}

var supportedSchemaKeywords = map[string]bool{
	"$ref": true, "type": true, "properties": true, "required": true, "additionalProperties": true,
	"items": true, "enum": true, "format": true, "uniqueItems": true, "default": true,
	"description": true, "deprecated": true, "title": true,
}

// validateValue checks value against schema. Stricter than OpenAPI: an object schema with
// properties and without additionalProperties allows only the declared properties.
func (spec *openAPISpec) validateValue(schema map[string]any, value any, at string, depth int) []string {
	if depth > maxOpenAPIDepth {
		return []string{at + ": schema nesting deeper than the limit"}
	}
	if ref, ok := schema["$ref"].(string); ok {
		resolved, err := spec.resolveLocalRef(ref)
		if err != nil {
			return []string{at + ": " + err.Error()}
		}
		return spec.validateValue(resolved, value, at, depth+1)
	}
	for keyword := range schema {
		if !supportedSchemaKeywords[keyword] {
			return []string{fmt.Sprintf("%s: schema keyword %q is not supported by this validator", at, keyword)}
		}
	}
	problems := make([]string, 0)
	schemaType, _ := schema["type"].(string)
	switch schemaType {
	case "object":
		object, ok := value.(map[string]any)
		if !ok {
			return []string{fmt.Sprintf("%s: got %T, want object", at, value)}
		}
		problems = append(problems, spec.validateObject(schema, object, at, depth)...)
	case "array":
		array, ok := value.([]any)
		if !ok {
			return []string{fmt.Sprintf("%s: got %T, want array", at, value)}
		}
		problems = append(problems, spec.validateArray(schema, array, at, depth)...)
	case "string":
		text, ok := value.(string)
		if !ok {
			return []string{fmt.Sprintf("%s: got %T, want string", at, value)}
		}
		problems = append(problems, checkStringFormat(schema, text, at)...)
	case "integer":
		number, ok := value.(json.Number)
		if !ok {
			return []string{fmt.Sprintf("%s: got %T, want integer", at, value)}
		}
		problems = append(problems, checkIntegerFormat(schema, number, at)...)
	case "boolean":
		if _, ok := value.(bool); !ok {
			return []string{fmt.Sprintf("%s: got %T, want boolean", at, value)}
		}
	case "":
		return []string{at + ": schema has no type"}
	default:
		return []string{fmt.Sprintf("%s: schema type %q is not supported by this validator", at, schemaType)}
	}
	if enumValues, hasEnum := schema["enum"].([]any); hasEnum {
		matched := false
		for _, allowed := range enumValues {
			if fmt.Sprint(allowed) == fmt.Sprint(value) {
				matched = true
			}
		}
		if !matched {
			problems = append(problems, fmt.Sprintf("%s: %v is not in enum %v", at, value, enumValues))
		}
	}
	return problems
}

func (spec *openAPISpec) validateObject(schema map[string]any, object map[string]any, at string, depth int) []string {
	problems := make([]string, 0)
	properties, hasProperties := schema["properties"].(map[string]any)
	if required, ok := schema["required"].([]any); ok {
		for _, name := range required {
			if _, present := object[fmt.Sprint(name)]; !present {
				problems = append(problems, fmt.Sprintf("%s: missing required property %q", at, name))
			}
		}
	}
	additional, hasAdditional := schema["additionalProperties"]
	names := make([]string, 0, len(object))
	for name := range object {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		propertyAt := at + "." + name
		if propertySchema, declared := properties[name].(map[string]any); declared {
			problems = append(problems, spec.validateValue(propertySchema, object[name], propertyAt, depth+1)...)
			continue
		}
		switch additionalSchema := additional.(type) {
		case map[string]any:
			problems = append(problems, spec.validateValue(additionalSchema, object[name], propertyAt, depth+1)...)
		case bool:
			if !additionalSchema {
				problems = append(problems, propertyAt+": additional property not allowed")
			}
		default:
			if hasProperties && !hasAdditional {
				problems = append(problems, propertyAt+": property not declared in the spec")
			}
		}
	}
	return problems
}

func (spec *openAPISpec) validateArray(schema map[string]any, array []any, at string, depth int) []string {
	problems := make([]string, 0)
	itemSchema, hasItems := schema["items"].(map[string]any)
	if !hasItems {
		return []string{at + ": array schema has no items"}
	}
	seen := make(map[string]bool, len(array))
	for index, item := range array {
		problems = append(problems, spec.validateValue(itemSchema, item, fmt.Sprintf("%s[%d]", at, index), depth+1)...)
		if unique, _ := schema["uniqueItems"].(bool); unique {
			encoded, _ := json.Marshal(item)
			if seen[string(encoded)] {
				problems = append(problems, fmt.Sprintf("%s[%d]: duplicate item in a uniqueItems array", at, index))
			}
			seen[string(encoded)] = true
		}
	}
	return problems
}

func checkStringFormat(schema map[string]any, text, at string) []string {
	format, _ := schema["format"].(string)
	switch format {
	case "":
		return nil
	case "date-time":
		if _, err := time.Parse(time.RFC3339Nano, text); err != nil {
			return []string{fmt.Sprintf("%s: %q is not an RFC 3339 date-time", at, text)}
		}
		return nil
	default:
		return []string{fmt.Sprintf("%s: string format %q is not supported by this validator", at, format)}
	}
}

func checkIntegerFormat(schema map[string]any, number json.Number, at string) []string {
	integer, err := strconv.ParseInt(number.String(), 10, 64)
	if err != nil {
		return []string{fmt.Sprintf("%s: %s is not an integer", at, number)}
	}
	format, _ := schema["format"].(string)
	bounds := map[string][2]int64{
		"int8":  {math.MinInt8, math.MaxInt8},
		"int32": {math.MinInt32, math.MaxInt32},
		"int64": {math.MinInt64, math.MaxInt64},
	}
	if format == "" {
		return nil
	}
	limits, known := bounds[format]
	if !known {
		return []string{fmt.Sprintf("%s: integer format %q is not supported by this validator", at, format)}
	}
	if integer < limits[0] || integer > limits[1] {
		return []string{fmt.Sprintf("%s: %d is out of range for %s", at, integer, format)}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Response conformance
// ---------------------------------------------------------------------------

var (
	exercisedOperationsMutex sync.Mutex
	exercisedOperations      = map[string]bool{}
)

// requireConformsToSpec validates a recorded response against the spec's schema for its operation
// and status. pathTemplate is the spec's path key, e.g. "/registry/metadata/v1/instruments/{instrumentId}".
func requireConformsToSpec(t *testing.T, recorder *httptest.ResponseRecorder, specFile, method, pathTemplate string) {
	t.Helper()
	spec := loadOpenAPISpec(t, specFile)
	schema, err := spec.responseSchema(pathTemplate, method, recorder.Code)
	if err != nil {
		t.Fatalf("response %d (%s): %v", recorder.Code, recorder.Body.String(), err)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("Content-Type %q, want application/json", contentType)
	}
	decoder := json.NewDecoder(bytes.NewReader(recorder.Body.Bytes()))
	decoder.UseNumber()
	var body any
	if err := decoder.Decode(&body); err != nil {
		t.Fatalf("response body is not JSON: %v: %s", err, recorder.Body.String())
	}
	if problems := spec.validateValue(schema, body, "$", 0); len(problems) > 0 {
		t.Fatalf("%s %s %d does not conform to %s:\n  %s\nbody: %s", method, pathTemplate, recorder.Code, specFile, strings.Join(problems, "\n  "), recorder.Body.String())
	}
	exercisedOperationsMutex.Lock()
	exercisedOperations[specFile+" "+method+" "+pathTemplate] = true
	exercisedOperationsMutex.Unlock()
}

// ---------------------------------------------------------------------------
// Tests of the reader and the validator themselves
// ---------------------------------------------------------------------------

func TestParseYAMLSubset_ReadsTheVendoredSpecs(t *testing.T) {
	cases := []struct {
		file           string
		wantOperations []string
	}{
		{specTokenMetadataV1, []string{
			"GET /registry/metadata/v1/info",
			"GET /registry/metadata/v1/instruments",
			"GET /registry/metadata/v1/instruments/{instrumentId}",
		}},
		{specTransferInstructionV1, []string{
			"POST /registry/transfer-instruction/v1/transfer-factory",
			"POST /registry/transfer-instruction/v1/{transferInstructionId}/choice-contexts/accept",
			"POST /registry/transfer-instruction/v1/{transferInstructionId}/choice-contexts/reject",
			"POST /registry/transfer-instruction/v1/{transferInstructionId}/choice-contexts/withdraw",
		}},
		{specAllocationInstructionV1, []string{
			"POST /registry/allocation-instruction/v1/allocation-factory",
		}},
		{specAllocationV1, []string{
			"POST /registry/allocations/v1/{allocationId}/choice-contexts/cancel",
			"POST /registry/allocations/v1/{allocationId}/choice-contexts/execute-transfer",
			"POST /registry/allocations/v1/{allocationId}/choice-contexts/withdraw",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			spec := loadOpenAPISpec(t, tc.file)
			got := spec.operations()
			want := append([]string(nil), tc.wantOperations...)
			sort.Strings(want)
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Fatalf("operations:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
		})
	}
}

// TestParseYAMLSubset_PinsSchemaFacts pins facts a reader bug would most likely corrupt.
func TestParseYAMLSubset_PinsSchemaFacts(t *testing.T) {
	transfer := loadOpenAPISpec(t, specTransferInstructionV1)
	disclosed, err := transfer.resolveLocalRef("#/components/schemas/DisclosedContract")
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(disclosed["required"]); got != "[templateId contractId createdEventBlob synchronizerId]" {
		t.Fatalf("DisclosedContract.required = %s", got)
	}
	kind, ok := lookupMapping(transfer.root, "components", "schemas", "TransferFactoryWithChoiceContext", "properties", "transferKind")
	if !ok || fmt.Sprint(kind["enum"]) != "[self direct offer]" {
		t.Fatalf("transferKind enum = %v", kind["enum"])
	}
	metadata := loadOpenAPISpec(t, specTokenMetadataV1)
	decimals, ok := lookupMapping(metadata.root, "components", "schemas", "Instrument", "properties", "decimals")
	if !ok || decimals["format"] != "int8" || decimals["default"] != int64(10) {
		t.Fatalf("Instrument.decimals = %v", decimals)
	}
	if _, ok := lookupMapping(metadata.root, "components", "responses", "404", "content", "application/json", "schema"); !ok {
		t.Fatal("components.responses.'404' did not parse as a quoted key")
	}
}

func TestParseYAMLSubset_RejectsConstructsOutsideTheSubset(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{"flow mapping", "a: {b: c}\n"},
		{"flow sequence", "a: [b, c]\n"},
		{"anchor", "a: &anchor b\n"},
		{"alias", "a: *anchor\n"},
		{"tag", "a: !tag b\n"},
		{"inline comment", "a: b # c\n"},
		{"multi-line plain scalar", "a: b\n  c\n"},
		{"duplicate key", "a: 1\na: 2\n"},
		{"tab indent", "a:\n\tb: c\n"},
		{"bare text", "a: 1\nnot a key\n"},
		{"empty", "\n# only a comment\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if parsed, err := parseYAMLSubset(tc.source); err == nil {
				t.Fatalf("parsed %q as %v, want an error", tc.source, parsed)
			}
		})
	}
}

func TestParseYAMLSubset_AcceptsTheSubset(t *testing.T) {
	source := "a:\n  b: 'x: y'\n  c: \"q\\\"\"\n  d:\n  - one\n  - k: v\n    l: 2\n  e: |\n    line one\n\n    line two\n  f: []\n  g: true\n"
	parsed, err := parseYAMLSubset(source)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(parsed)
	want := `{"a":{"b":"x: y","c":"q\"","d":["one",{"k":"v","l":2}],"e":"line one\n\nline two","f":[],"g":true}}`
	if string(encoded) != want {
		t.Fatalf("got  %s\nwant %s", encoded, want)
	}
}

func TestValidateValue_RejectsNonConformingValues(t *testing.T) {
	spec := loadOpenAPISpec(t, specTransferInstructionV1)
	schema, err := spec.responseSchema("/registry/transfer-instruction/v1/transfer-factory", "POST", 200)
	if err != nil {
		t.Fatal(err)
	}
	valid := `{"factoryId":"00ab","transferKind":"offer","choiceContext":{"choiceContextData":{"values":{}},"disclosedContracts":[{"templateId":"p:M:E","contractId":"00ab","createdEventBlob":"AA==","synchronizerId":"s::1","debugCreatedAt":"2026-11-18T10:00:00Z"}]}}`
	cases := []struct {
		name     string
		body     string
		wantText string
	}{
		{"valid", valid, ""},
		{"missing required", `{"factoryId":"00ab","transferKind":"offer"}`, "missing required property \"choiceContext\""},
		{"bad enum", strings.Replace(valid, `"offer"`, `"push"`, 1), "not in enum"},
		{"wrong type", strings.Replace(valid, `"factoryId":"00ab"`, `"factoryId":7`, 1), "want string"},
		{"undeclared property", strings.Replace(valid, `"factoryId"`, `"extra":1,"factoryId"`, 1), "property not declared"},
		{"bad date-time", strings.Replace(valid, "2026-11-18T10:00:00Z", "yesterday", 1), "RFC 3339"},
		{"missing synchronizerId", strings.Replace(valid, `,"synchronizerId":"s::1"`, "", 1), "synchronizerId"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decoder := json.NewDecoder(strings.NewReader(tc.body))
			decoder.UseNumber()
			var body any
			if err := decoder.Decode(&body); err != nil {
				t.Fatal(err)
			}
			problems := strings.Join(spec.validateValue(schema, body, "$", 0), "\n")
			if tc.wantText == "" && problems != "" {
				t.Fatalf("valid body rejected: %s", problems)
			}
			if tc.wantText != "" && !strings.Contains(problems, tc.wantText) {
				t.Fatalf("problems %q do not mention %q", problems, tc.wantText)
			}
		})
	}
}

func TestValidateValue_IntegerFormatBounds(t *testing.T) {
	spec := loadOpenAPISpec(t, specTokenMetadataV1)
	instrument, err := spec.resolveLocalRef("#/components/schemas/Instrument")
	if err != nil {
		t.Fatal(err)
	}
	decimals, _ := lookupMapping(instrument, "properties", "decimals")
	cases := []struct {
		value   string
		wantErr bool
	}{{"0", false}, {"10", false}, {"127", false}, {"128", true}, {"-129", true}, {"1.5", true}}
	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			problems := spec.validateValue(decimals, json.Number(tc.value), "$", 0)
			if (len(problems) > 0) != tc.wantErr {
				t.Fatalf("value %s: problems %v, wantErr %v", tc.value, problems, tc.wantErr)
			}
		})
	}
}
