package daemon

import (
	"runtime"
	"strings"
	"testing"
)

func TestValidateInterfaceName(t *testing.T) {
	t.Run("empty string uses default", func(t *testing.T) {
		if err := ValidateInterfaceName(""); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	// ── Path traversal (all platforms) ──────────────────────────
	pathTraversal := []struct {
		name   string
		input  string
		errMsg string
	}{
		{"dotdot", "..", "must not be"},
		{"single dot", ".", "must not be"},
		{"contains slash", "foo/bar", "path separators"},
		{"contains backslash", `foo\bar`, "path separators"},
		{"traversal sequence", "../etc", "path separators"},
	}
	for _, tt := range pathTraversal {
		t.Run("path_traversal/"+tt.name, func(t *testing.T) {
			err := ValidateInterfaceName(tt.input)
			if err == nil {
				t.Fatalf("expected error for %q", tt.input)
			}
			if !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("error %q missing substring %q", err, tt.errMsg)
			}
		})
	}

	// ── Null bytes (all platforms) ──────────────────────────────
	t.Run("null byte", func(t *testing.T) {
		err := ValidateInterfaceName("wg0\x00evil")
		if err == nil {
			t.Fatal("expected error for null byte")
		}
		if !strings.Contains(err.Error(), "null bytes") {
			t.Errorf("error %q missing 'null bytes'", err)
		}
	})

	// ── Shell metacharacters (all platforms) ─────────────────────
	shellMeta := []struct {
		name  string
		input string
	}{
		{"semicolon", "wg0;rm -rf /"},
		{"dollar", "wg0$(cmd)"},
		{"backtick", "wg0`cmd`"},
		{"single quote", "wg0'evil"},
		{"double quote", `wg0"evil`},
		{"pipe", "wg0|cat"},
		{"ampersand", "wg0&bg"},
		{"space", "wg 0"},
	}
	for _, tt := range shellMeta {
		t.Run("shell_meta/"+tt.name, func(t *testing.T) {
			if err := ValidateInterfaceName(tt.input); err == nil {
				t.Errorf("expected error for %q", tt.input)
			}
		})
	}
}

func TestValidateInterfaceName_Linux(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("Linux-specific tests")
	}

	valid := []struct {
		name  string
		input string
	}{
		{"default wg0", "wg0"},
		{"custom cloudroof0", "cloudroof0"},
		{"hyphenated mesh-1", "mesh-1"},
		{"underscore corp_vpn", "corp_vpn"},
		{"single letter", "w"},
		{"max 15 chars", "abcdefghijklmno"},
	}
	for _, tt := range valid {
		t.Run("valid/"+tt.name, func(t *testing.T) {
			if err := ValidateInterfaceName(tt.input); err != nil {
				t.Errorf("unexpected error for %q: %v", tt.input, err)
			}
		})
	}

	invalid := []struct {
		name   string
		input  string
		errMsg string
	}{
		{"16 chars too long", "abcdefghijklmnop", "maximum is 15"},
		{"starts with digit", "0wg", "must start with a letter"},
		{"starts with hyphen", "-custom", "must start with a letter"},
		{"starts with underscore", "_iface", "must start with a letter"},
	}
	for _, tt := range invalid {
		t.Run("invalid/"+tt.name, func(t *testing.T) {
			err := ValidateInterfaceName(tt.input)
			if err == nil {
				t.Fatalf("expected error for %q", tt.input)
			}
			if !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("error %q missing substring %q", err, tt.errMsg)
			}
		})
	}
}

func TestValidateInterfaceName_Darwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS-specific tests")
	}

	valid := []struct {
		name  string
		input string
	}{
		{"utun20", "utun20"},
		{"utun0", "utun0"},
		{"utun999", "utun999"},
	}
	for _, tt := range valid {
		t.Run("valid/"+tt.name, func(t *testing.T) {
			if err := ValidateInterfaceName(tt.input); err != nil {
				t.Errorf("unexpected error for %q: %v", tt.input, err)
			}
		})
	}

	invalid := []struct {
		name   string
		input  string
		errMsg string
	}{
		{"non-utun wg0", "wg0", "utun<N>"},
		{"non-utun custom", "cloudroof0", "utun<N>"},
		{"utun without number", "utun", "utun<N>"},
		{"utun with text", "utunXYZ", "utun<N>"},
		{"starts with digit", "0wg", "utun<N>"},
		{"starts with hyphen", "-custom", "utun<N>"},
	}
	for _, tt := range invalid {
		t.Run("invalid/"+tt.name, func(t *testing.T) {
			err := ValidateInterfaceName(tt.input)
			if err == nil {
				t.Fatalf("expected error for %q", tt.input)
			}
			if !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("error %q missing substring %q", err, tt.errMsg)
			}
		})
	}
}
