package eidosmeta

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		contents  string
		want      Meta
		wantError bool
	}{
		{
			name: "valid implemented",
			contents: `---
status: implemented
compat-dimensions:
  - cli
  - wire
tracking-issue: "123"
since: "v1.2.3"
---
# Implemented
`,
			want: Meta{
				Status:           StatusImplemented,
				CompatDimensions: []string{"cli", "wire"},
				TrackingIssue:    "123",
				Since:            "v1.2.3",
			},
		},
		{
			name: "valid provisional",
			contents: `---
status: provisional
compat-dimensions:
  - api
tracking-issue: "456"
since: ""
---
# Provisional
`,
			want: Meta{
				Status:           StatusProvisional,
				CompatDimensions: []string{"api"},
				TrackingIssue:    "456",
			},
		},
		{
			name: "valid with legacy frontmatter keys",
			contents: `---
status: implemented
compat-dimensions: []
tldr: Existing short summary
category: core
---
# Legacy keys
`,
			want: Meta{Status: StatusImplemented, CompatDimensions: []string{}},
		},
		{
			name: "missing status",
			contents: `---
compat-dimensions: []
---
# Missing status
`,
			wantError: true,
		},
		{
			name: "invalid status value",
			contents: `---
status: done
compat-dimensions: []
---
# Invalid status
`,
			wantError: true,
		},
		{
			name: "empty compat-dimensions",
			contents: `---
status: implemented
compat-dimensions: []
---
# Empty dimensions
`,
			want: Meta{Status: StatusImplemented, CompatDimensions: []string{}},
		},
		{
			name: "nil compat-dimensions",
			contents: `---
status: implemented
---
# Nil dimensions
`,
			want: Meta{Status: StatusImplemented},
		},
		{
			name: "missing tracking-issue",
			contents: `---
status: provisional
compat-dimensions: []
since: ""
---
# Optional issue
`,
			want: Meta{Status: StatusProvisional, CompatDimensions: []string{}},
		},
		{
			name: "malformed YAML frontmatter",
			contents: `---
status: [implemented
compat-dimensions: []
---
# Malformed
`,
			wantError: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "feature.md")
			if err := os.WriteFile(path, []byte(tt.contents), 0o644); err != nil {
				t.Fatalf("writing test file: %v", err)
			}

			got, diags := ParseFile(path)
			hasError := hasErrorDiag(diags)
			if hasError != tt.wantError {
				t.Fatalf("ParseFile() error diag = %v, want %v; diags = %#v", hasError, tt.wantError, diags)
			}
			if tt.wantError {
				return
			}
			if got.Status != tt.want.Status {
				t.Fatalf("Status = %q, want %q", got.Status, tt.want.Status)
			}
			if got.TrackingIssue != tt.want.TrackingIssue {
				t.Fatalf("TrackingIssue = %q, want %q", got.TrackingIssue, tt.want.TrackingIssue)
			}
			if got.Since != tt.want.Since {
				t.Fatalf("Since = %q, want %q", got.Since, tt.want.Since)
			}
			if len(got.CompatDimensions) != len(tt.want.CompatDimensions) {
				t.Fatalf("CompatDimensions = %#v, want %#v", got.CompatDimensions, tt.want.CompatDimensions)
			}
			for i := range got.CompatDimensions {
				if got.CompatDimensions[i] != tt.want.CompatDimensions[i] {
					t.Fatalf("CompatDimensions = %#v, want %#v", got.CompatDimensions, tt.want.CompatDimensions)
				}
			}
		})
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		meta      Meta
		wantError bool
	}{
		{
			name: "implemented",
			meta: Meta{Status: StatusImplemented},
		},
		{
			name: "provisional",
			meta: Meta{Status: StatusProvisional},
		},
		{
			name:      "missing status",
			meta:      Meta{},
			wantError: true,
		},
		{
			name:      "invalid status",
			meta:      Meta{Status: Status("done")},
			wantError: true,
		},
		{
			name: "empty compat dimensions",
			meta: Meta{Status: StatusImplemented, CompatDimensions: []string{}},
		},
		{
			name: "nil compat dimensions",
			meta: Meta{Status: StatusImplemented},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			diags := Validate(tt.meta)
			hasError := hasErrorDiag(diags)
			if hasError != tt.wantError {
				t.Fatalf("Validate() error diag = %v, want %v; diags = %#v", hasError, tt.wantError, diags)
			}
		})
	}
}

func hasErrorDiag(diags []Diag) bool {
	for _, diag := range diags {
		if diag.Severity == "error" {
			return true
		}
	}
	return false
}
