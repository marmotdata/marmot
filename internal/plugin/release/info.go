package release

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// infoBundle holds the payloads that ship as layers on the info
// referrer. Any element may be zero-length; empty layers are skipped.
type infoBundle struct {
	Readme       []byte
	Metadata     []byte
	AssetSchemas []byte
}

// buildInfoBundle assembles README, metadata (via the plugin's
// --dump-metadata), and asset schemas (via source AST scan). The
// binary is required — the registry needs metadata to render a page.
func buildInfoBundle(ctx context.Context, name, source, dist string) (infoBundle, error) {
	var b infoBundle

	if readme, err := os.ReadFile(filepath.Join(source, "README.md")); err == nil {
		b.Readme = readme
	} else if !os.IsNotExist(err) {
		return infoBundle{}, fmt.Errorf("read README: %w", err)
	}

	binary, err := findBinary(dist, name, Platform{OS: runtime.GOOS, Arch: runtime.GOARCH})
	if err != nil {
		return infoBundle{}, fmt.Errorf("locate binary for --dump-metadata: %w", err)
	}
	meta, err := exec.CommandContext(ctx, binary, "--dump-metadata").Output()
	if err != nil {
		return infoBundle{}, fmt.Errorf("run %s --dump-metadata: %w", binary, errWithStderr(err))
	}
	b.Metadata = normaliseTrailingNewline(meta)

	schemas, err := ScanAssetSchemas(source)
	if err != nil {
		return infoBundle{}, fmt.Errorf("scan asset schemas: %w", err)
	}
	if len(schemas) > 0 {
		encoded, err := json.MarshalIndent(schemas, "", "  ")
		if err != nil {
			return infoBundle{}, err
		}
		b.AssetSchemas = normaliseTrailingNewline(encoded)
	}

	return b, nil
}

// errWithStderr appends the ExitError's stderr tail so CI logs are useful.
func errWithStderr(err error) error {
	var ee *exec.ExitError
	if errors.As(err, &ee) && len(ee.Stderr) > 0 {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(ee.Stderr)))
	}
	return err
}

func normaliseTrailingNewline(b []byte) []byte {
	return append([]byte(strings.TrimRight(string(b), " \n\t\r")), '\n')
}
