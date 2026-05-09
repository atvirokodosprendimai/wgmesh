package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/atvirokodosprendimai/wgmesh/eidos/eidosmeta"
)

func main() {
	features, diags := DiscoverFeatures(".")
	hasError := false
	for _, diag := range diags {
		if diag.Severity == "error" {
			hasError = true
		}
		fmt.Printf("%s:%d: %s: %s\n", diag.File, diag.Line, diag.Severity, diag.Message)
	}
	if hasError {
		os.Exit(1)
	}

	if err := os.WriteFile("STATUS.md", []byte(RenderStatus(features)), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "writing STATUS.md: %v\n", err)
		os.Exit(1)
	}
}

func DiscoverFeatures(root string) ([]FeatureStatus, []eidosmeta.Diag) {
	paths, err := filepath.Glob(filepath.Join(root, "eidos", "*.md"))
	if err != nil {
		return nil, []eidosmeta.Diag{{Severity: "error", Message: fmt.Sprintf("glob eidos/*.md: %v", err)}}
	}
	sort.Strings(paths)

	openAPIFiles, err := findOpenAPIFiles(root)
	if err != nil {
		return nil, []eidosmeta.Diag{{Severity: "error", Message: fmt.Sprintf("finding openapi.yaml: %v", err)}}
	}

	var (
		features []FeatureStatus
		diags    []eidosmeta.Diag
	)
	for _, path := range paths {
		meta, fileDiags := eidosmeta.ParseFile(path)
		diags = append(diags, fileDiags...)
		if hasErrorDiag(fileDiags) {
			continue
		}

		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		slug := featureSlug(name)
		feature := FeatureStatus{
			Name:          name,
			Slug:          slug,
			Status:        meta.Status,
			TrackingIssue: meta.TrackingIssue,
			Since:         meta.Since,
		}
		for _, dimension := range meta.CompatDimensions {
			feature.Dimensions = append(feature.Dimensions, checkDimension(root, slug, name, dimension, openAPIFiles))
		}
		sort.Slice(feature.Dimensions, func(i, j int) bool {
			return feature.Dimensions[i].Name < feature.Dimensions[j].Name
		})
		features = append(features, feature)
	}

	return features, diags
}

func checkDimension(root, slug, name, dimension string, openAPIFiles []string) DimensionStatus {
	switch dimension {
	case "cli", "behavior":
		pattern := filepath.Join(root, "testdata", "script", slug+"*.txtar")
		matches, err := filepath.Glob(pattern)
		if err == nil && len(matches) > 0 {
			sort.Strings(matches)
			return DimensionStatus{Name: dimension, Present: true, Note: trimRoot(root, matches[0])}
		}
		return DimensionStatus{Name: dimension, Present: false, Note: fmt.Sprintf("missing testdata/script/%s*.txtar", slug)}
	case "wire":
		pattern := filepath.Join(root, "testdata", "compat", slug, "v*")
		matches, err := filepath.Glob(pattern)
		if err == nil {
			for _, match := range matches {
				if info, statErr := os.Stat(match); statErr == nil && info.IsDir() {
					return DimensionStatus{Name: dimension, Present: true, Note: trimRoot(root, match)}
				}
			}
		}
		return DimensionStatus{Name: dimension, Present: false, Note: fmt.Sprintf("missing testdata/compat/%s/v*/", slug)}
	case "api":
		for _, path := range openAPIFiles {
			data, err := os.ReadFile(path)
			if err == nil && openAPIReferencesFeature(data, slug, name) {
				return DimensionStatus{Name: dimension, Present: true, Note: trimRoot(root, path)}
			}
		}
		return DimensionStatus{Name: dimension, Present: false, Note: "openapi.yaml not found or does not reference feature"}
	default:
		return DimensionStatus{Name: dimension, Present: false, Note: "unknown dimension"}
	}
}

func findOpenAPIFiles(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".compound-engineering":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(d.Name(), "openapi.yaml") {
			paths = append(paths, path)
		}
		return nil
	})
	sort.Strings(paths)
	return paths, err
}

func openAPIReferencesFeature(data []byte, slug, name string) bool {
	haystack := bytes.ToLower(data)
	candidates := []string{
		slug,
		strings.ToLower(name),
		strings.ReplaceAll(strings.ToLower(name), " ", "-"),
	}
	for _, candidate := range candidates {
		if candidate != "" && bytes.Contains(haystack, []byte(candidate)) {
			return true
		}
	}
	return false
}

func featureSlug(name string) string {
	name = strings.ToLower(name)
	return strings.Join(strings.Fields(name), "-")
}

func trimRoot(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}

func hasErrorDiag(diags []eidosmeta.Diag) bool {
	for _, diag := range diags {
		if diag.Severity == "error" {
			return true
		}
	}
	return false
}
