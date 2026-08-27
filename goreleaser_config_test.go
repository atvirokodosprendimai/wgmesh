package main

import (
	"os"
	"strings"
	"testing"
)

// TestGoreleaserBrewsService guards the GoReleaser brew config that generates
// the Homebrew formula (issue #825): the formula must keep a launchd service
// block with the wgmesh daemon, macOS WireGuard dependencies, and writable
// state/secret directories. It must NOT regress to a bare bin.install formula.
func TestGoreleaserBrewsService(t *testing.T) {
	data, err := os.ReadFile(".goreleaser.yml")
	if err != nil {
		t.Fatalf("read .goreleaser.yml: %v", err)
	}
	brews := extractSection(string(data), "brews:")
	if brews == "" {
		t.Fatal("brews section not found in .goreleaser.yml")
	}

	want := []string{
		"service: |",
		`run [opt_bin/"wgmesh", "join", "--state-dir", var/"wgmesh"]`,
		"keep_alive true",
		"run_type :immediate",
		"require_root true",
		`environment_variables({"WGMESH_SECRET_FILE" => etc/"wgmesh"/"secret"})`,
		"- name: wireguard-go",
		"- name: wireguard-tools",
		`(etc/"wgmesh").mkpath`,
		`(var/"wgmesh").mkpath`,
		"caveats:",
	}
	for _, s := range want {
		if !strings.Contains(brews, s) {
			t.Errorf("brews section missing %q", s)
		}
	}
}

// extractSection returns the text from the first occurrence of header up to
// (not including) the next column-0 key, or the end of s.
func extractSection(s, header string) string {
	idx := strings.Index(s, header)
	if idx < 0 {
		return ""
	}
	rest := s[idx:]
	nl := strings.IndexByte(rest, 10)
	for nl >= 0 {
		if nl+1 < len(rest) && rest[nl+1] != ' ' && rest[nl+1] != '	' {
			return rest[:nl+1]
		}
		next := strings.IndexByte(rest[nl+1:], 10)
		if next < 0 {
			break
		}
		nl = nl + 1 + next
	}
	return rest
}
