package eidosmeta

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
)

type Status string

const (
	StatusImplemented Status = "implemented"
	StatusProvisional Status = "provisional"
)

type Meta struct {
	Status              Status
	CompatDimensions    []string
	TrackingIssue       string
	Since               string
	CompatDimensionsSet bool
	TrackingIssueSet    bool
	SinceSet            bool
}

type Diag struct {
	Severity string
	File     string
	Line     int
	Message  string
}

var validCompatDimensions = map[string]struct{}{
	"api":      {},
	"behavior": {},
	"cli":      {},
	"wire":     {},
}

func Validate(meta Meta) []Diag {
	var diags []Diag
	switch meta.Status {
	case StatusImplemented, StatusProvisional:
	case "":
		diags = append(diags, Diag{Severity: "error", Message: "missing status"})
	default:
		diags = append(diags, Diag{Severity: "error", Message: fmt.Sprintf("invalid status %q", meta.Status)})
	}
	if !meta.CompatDimensionsSet {
		diags = append(diags, Diag{Severity: "error", Message: "missing compat-dimensions"})
	}
	if !meta.TrackingIssueSet {
		diags = append(diags, Diag{Severity: "error", Message: "missing tracking-issue"})
	}
	if !meta.SinceSet {
		diags = append(diags, Diag{Severity: "error", Message: "missing since"})
	}

	for _, dimension := range meta.CompatDimensions {
		if _, ok := validCompatDimensions[dimension]; !ok {
			diags = append(diags, Diag{
				Severity: "error",
				Message:  fmt.Sprintf("invalid compat-dimension %q", dimension),
			})
		}
	}

	return diags
}

func ParseFile(path string) (Meta, []Diag) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Meta{}, []Diag{{Severity: "error", File: path, Message: fmt.Sprintf("reading file: %v", err)}}
	}

	meta, diags := parseFrontmatter(path, string(data))
	for i := range diags {
		if diags[i].File == "" {
			diags[i].File = path
		}
	}

	validation := Validate(meta)
	for i := range validation {
		validation[i].File = path
	}
	diags = append(diags, validation...)

	return meta, diags
}

func parseFrontmatter(path, contents string) (Meta, []Diag) {
	scanner := bufio.NewScanner(strings.NewReader(contents))
	if !scanner.Scan() {
		return Meta{}, []Diag{{Severity: "error", File: path, Line: 1, Message: "missing YAML frontmatter"}}
	}
	if strings.TrimSpace(scanner.Text()) != "---" {
		return Meta{}, []Diag{{Severity: "error", File: path, Line: 1, Message: "missing YAML frontmatter"}}
	}

	var (
		lines      []frontmatterLine
		lineNumber = 1
	)
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			meta, diags := parseYAMLLines(lines)
			return meta, withFile(path, diags)
		}
		lines = append(lines, frontmatterLine{number: lineNumber, text: line})
	}
	if err := scanner.Err(); err != nil {
		return Meta{}, []Diag{{Severity: "error", File: path, Line: lineNumber, Message: fmt.Sprintf("reading frontmatter: %v", err)}}
	}
	return Meta{}, []Diag{{Severity: "error", File: path, Line: lineNumber, Message: "unterminated YAML frontmatter"}}
}

type frontmatterLine struct {
	number int
	text   string
}

func parseYAMLLines(lines []frontmatterLine) (Meta, []Diag) {
	var (
		meta          Meta
		diags         []Diag
		currentSeqKey string
		seenKeys      = map[string]struct{}{}
	)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line.text)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.HasPrefix(trimmed, "- ") {
			if currentSeqKey != "compat-dimensions" {
				diags = append(diags, Diag{Severity: "error", Line: line.number, Message: "sequence item without sequence key"})
				continue
			}
			value, ok := parseScalar(strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
			if !ok {
				diags = append(diags, Diag{Severity: "error", Line: line.number, Message: "malformed compat-dimensions item"})
				continue
			}
			meta.CompatDimensions = append(meta.CompatDimensions, value)
			continue
		}

		currentSeqKey = ""
		key, rawValue, ok := strings.Cut(trimmed, ":")
		if !ok {
			diags = append(diags, Diag{Severity: "error", Line: line.number, Message: "malformed YAML mapping"})
			continue
		}
		key = strings.TrimSpace(key)
		rawValue = strings.TrimSpace(rawValue)
		if key == "" {
			diags = append(diags, Diag{Severity: "error", Line: line.number, Message: "empty YAML key"})
			continue
		}
		if _, ok := seenKeys[key]; ok {
			diags = append(diags, Diag{Severity: "error", Line: line.number, Message: fmt.Sprintf("duplicate key %q", key)})
			continue
		}
		seenKeys[key] = struct{}{}

		switch key {
		case "status":
			value, ok := parseScalar(rawValue)
			if !ok || value == "" {
				diags = append(diags, Diag{Severity: "error", Line: line.number, Message: "malformed status"})
				continue
			}
			meta.Status = Status(value)
		case "compat-dimensions":
			meta.CompatDimensionsSet = true
			if rawValue == "" {
				currentSeqKey = key
				continue
			}
			values, ok := parseInlineStringList(rawValue)
			if !ok {
				diags = append(diags, Diag{Severity: "error", Line: line.number, Message: "malformed compat-dimensions"})
				continue
			}
			meta.CompatDimensions = values
		case "tracking-issue":
			meta.TrackingIssueSet = true
			value, ok := parseScalar(rawValue)
			if !ok {
				diags = append(diags, Diag{Severity: "error", Line: line.number, Message: "malformed tracking-issue"})
				continue
			}
			meta.TrackingIssue = value
		case "since":
			meta.SinceSet = true
			value, ok := parseScalar(rawValue)
			if !ok {
				diags = append(diags, Diag{Severity: "error", Line: line.number, Message: "malformed since"})
				continue
			}
			meta.Since = value
		default:
			continue
		}
	}

	return meta, diags
}

func parseScalar(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", true
	}
	if strings.HasPrefix(raw, "[") || strings.HasPrefix(raw, "{") {
		return "", false
	}
	if strings.HasPrefix(raw, "\"") || strings.HasPrefix(raw, "'") {
		if len(raw) < 2 || raw[len(raw)-1] != raw[0] {
			return "", false
		}
		return raw[1 : len(raw)-1], true
	}
	return raw, !strings.ContainsAny(raw, "[]{}")
}

func parseInlineStringList(raw string) ([]string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "[]" {
		return []string{}, true
	}
	if !strings.HasPrefix(raw, "[") || !strings.HasSuffix(raw, "]") {
		return nil, false
	}

	body := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(raw, "["), "]"))
	if body == "" {
		return []string{}, true
	}

	parts := strings.Split(body, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value, ok := parseScalar(strings.TrimSpace(part))
		if !ok || value == "" {
			return nil, false
		}
		values = append(values, value)
	}
	return values, true
}

func withFile(path string, diags []Diag) []Diag {
	for i := range diags {
		diags[i].File = path
	}
	return diags
}

func ValidCompatDimensions() []string {
	dimensions := make([]string, 0, len(validCompatDimensions))
	for dimension := range validCompatDimensions {
		dimensions = append(dimensions, dimension)
	}
	sort.Strings(dimensions)
	return dimensions
}
