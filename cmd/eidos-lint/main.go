package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/atvirokodosprendimai/wgmesh/eidos/eidosmeta"
)

func main() {
	paths, err := filepath.Glob("eidos/*.md")
	if err != nil {
		fmt.Fprintf(os.Stderr, "glob eidos/*.md: %v\n", err)
		os.Exit(1)
	}

	hasError := false
	for _, path := range paths {
		_, diags := eidosmeta.ParseFile(path)
		for _, diag := range diags {
			if diag.Severity == "error" {
				hasError = true
			}
			fmt.Printf("%s:%d: %s: %s\n", diag.File, diag.Line, diag.Severity, diag.Message)
		}
	}
	if hasError {
		os.Exit(1)
	}
}
