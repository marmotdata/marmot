package main

import (
	"os"
	"path/filepath"
	"strings"
	_ "time/tzdata"

	"github.com/marmotdata/marmot/internal/cmd"
)

// Docker runs this name for registries mapped to the marmot credential
// helper. A symlink to the marmot binary under that name is the whole install.
const credentialHelperPrefix = "docker-credential-marmot" //nolint:gosec // G101: a helper binary name, not a credential

func main() {
	if strings.HasPrefix(filepath.Base(os.Args[0]), credentialHelperPrefix) {
		os.Args = append([]string{os.Args[0], "credential-helper"}, os.Args[1:]...)
	}

	// Cobra already printed the error.
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
