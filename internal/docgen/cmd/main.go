package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/marmotdata/marmot/internal/docgen"
)

func main() {
	pluginDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting working directory: %v\n", err)
		os.Exit(1)
	}

	docsPath := filepath.Join(pluginDir, "..", "..", "..", "..", "web", "docs", "docs")
	fmt.Printf("Generating docs for plugin in: %s\n", pluginDir)

	if err := docgen.GeneratePluginDocs(pluginDir, docsPath); err != nil {
		fmt.Printf("Error generating docs: %v\n", err)
		os.Exit(1)
	}
}
