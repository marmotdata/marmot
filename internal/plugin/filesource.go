package plugin

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/rs/zerolog/log"
	"sigs.k8s.io/yaml"
)

// FileSourceConfig configures how a plugin resolves file paths from local, S3 or Git sources.
type FileSourceConfig struct {
	SourceType string           `json:"source_type,omitempty" description:"File source backend (auto-detected from path when empty)" validate:"omitempty,oneof=local s3 git" default:"local" hidden:"true"`
	S3Source   *S3SourceConfig  `json:"s3_source,omitempty" description:"S3 file source configuration" show_when:"source_type:s3"`
	GitSource  *GitSourceConfig `json:"git_source,omitempty" description:"Git repository file source configuration" show_when:"source_type:git"`
}

// S3SourceConfig holds configuration for downloading files from S3.
type S3SourceConfig struct {
	Credentials AWSCredentials `json:"credentials" description:"AWS credentials for S3 access"`
}

// GitSourceConfig holds configuration for cloning a Git repository.
type GitSourceConfig struct {
	URL        string `json:"url" description:"Git repository URL"`
	Ref        string `json:"ref,omitempty" description:"Branch, tag or commit to check out" default:"main"`
	Path       string `json:"path,omitempty" description:"Subdirectory within the repository"`
	Token      string `json:"token,omitempty" description:"Personal access token for HTTPS auth" sensitive:"true"`
	SSHKeyPath string `json:"ssh_key_path,omitempty" description:"Path to SSH private key for SSH auth"`
}

// ResolveFilePath resolves a path to a local filesystem path, S3 or git.
func ResolveFilePath(ctx context.Context, fsc *FileSourceConfig, path string) (string, func(), error) {
	noop := func() {}

	sourceType := "local"
	if fsc != nil && fsc.SourceType != "" {
		sourceType = fsc.SourceType
	} else {
		sourceType = DetectSourceType(path)
	}

	switch sourceType {
	case "s3":
		return resolveS3Path(ctx, fsc, path)
	case "git":
		return resolveGitPath(ctx, fsc, path)
	default:
		return path, noop, nil
	}
}

// DetectSourceType infers the file source backend from a path prefix.
func DetectSourceType(path string) string {
	if strings.HasPrefix(path, "s3://") {
		return "s3"
	}
	if strings.HasPrefix(path, "git::") {
		return "git"
	}
	return "local"
}

// ParseS3Path extracts bucket and key from an s3:// URI.
func ParseS3Path(path string) (bucket, key string, err error) {
	if !strings.HasPrefix(path, "s3://") {
		return "", "", fmt.Errorf("not an S3 path: %s", path)
	}
	trimmed := strings.TrimPrefix(path, "s3://")
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) == 0 || parts[0] == "" {
		return "", "", fmt.Errorf("invalid S3 path, missing bucket: %s", path)
	}
	bucket = parts[0]
	if len(parts) == 2 {
		key = parts[1]
	}
	return bucket, key, nil
}

// ParseGitPath extracts URL, subpath and ref from a git:: prefixed path.
// Format: git::<url>[//<subpath>][?ref=<ref>]
func ParseGitPath(path string) (url, subPath, ref string) {
	trimmed := strings.TrimPrefix(path, "git::")

	// Extract ref from query parameter
	if idx := strings.Index(trimmed, "?ref="); idx != -1 {
		ref = trimmed[idx+5:]
		trimmed = trimmed[:idx]
	}

	// Extract subpath after // separator, but skip the :// in scheme (e.g. https://)
	// Look for // that isn't preceded by ':'
	searchFrom := 0
	if schemeIdx := strings.Index(trimmed, "://"); schemeIdx != -1 {
		searchFrom = schemeIdx + 3
	}
	if idx := strings.Index(trimmed[searchFrom:], "//"); idx != -1 {
		actualIdx := searchFrom + idx
		subPath = trimmed[actualIdx+2:]
		trimmed = trimmed[:actualIdx]
	}

	url = trimmed
	return url, subPath, ref
}

// ExtractFileSourceConfig extracts a FileSourceConfig from raw plugin config.
func ExtractFileSourceConfig(rawConfig map[string]interface{}) (*FileSourceConfig, error) {
	var fsc FileSourceConfig
	configBytes, err := yaml.Marshal(rawConfig)
	if err != nil {
		return nil, fmt.Errorf("marshaling raw config: %w", err)
	}
	if err := yaml.Unmarshal(configBytes, &fsc); err != nil {
		return nil, fmt.Errorf("unmarshaling into FileSourceConfig: %w", err)
	}
	return &fsc, nil
}

func resolveS3Path(ctx context.Context, fsc *FileSourceConfig, path string) (string, func(), error) {
	noop := func() {}

	var creds AWSCredentials

	if fsc != nil && fsc.S3Source != nil {
		creds = fsc.S3Source.Credentials
	}

	if !strings.HasPrefix(path, "s3://") {
		return "", noop, fmt.Errorf("S3 path must start with s3://")
	}

	bucket, key, err := ParseS3Path(path)
	if err != nil {
		return "", noop, err
	}

	awsCfg := &AWSConfig{Credentials: creds}
	awsConfig, err := awsCfg.NewAWSConfig(ctx)
	if err != nil {
		return "", noop, fmt.Errorf("creating AWS config: %w", err)
	}

	client := s3.NewFromConfig(awsConfig, func(o *s3.Options) {
		if awsConfig.BaseEndpoint != nil {
			o.UsePathStyle = true
			o.BaseEndpoint = awsConfig.BaseEndpoint
		}
	})

	tmpDir, err := os.MkdirTemp("", "marmot-s3-*")
	if err != nil {
		return "", noop, fmt.Errorf("creating temp directory: %w", err)
	}
	cleanup := func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			log.Warn().Err(err).Str("dir", tmpDir).Msg("Failed to clean up temp directory")
		}
	}

	if key != "" && !strings.HasSuffix(key, "/") {
		// Single file download
		localPath := filepath.Join(tmpDir, filepath.Base(key))
		if err := downloadS3Object(ctx, client, bucket, key, localPath); err != nil {
			cleanup()
			return "", noop, fmt.Errorf("downloading s3://%s/%s: %w", bucket, key, err)
		}
		return localPath, cleanup, nil
	}

	// Prefix/directory download
	if err := downloadS3Prefix(ctx, client, bucket, key, tmpDir); err != nil {
		cleanup()
		return "", noop, fmt.Errorf("downloading s3://%s/%s: %w", bucket, key, err)
	}

	return tmpDir, cleanup, nil
}

func downloadS3Object(ctx context.Context, client *s3.Client, bucket, key, localPath string) error {
	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	output, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("getting object: %w", err)
	}
	defer output.Body.Close()

	f, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("creating file %s: %w", localPath, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, output.Body); err != nil {
		return fmt.Errorf("writing file %s: %w", localPath, err)
	}

	return nil
}

func downloadS3Prefix(ctx context.Context, client *s3.Client, bucket, prefix, tmpDir string) error {
	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("listing objects: %w", err)
		}

		for _, obj := range page.Contents {
			if obj.Key == nil {
				continue
			}
			// Preserve directory structure relative to prefix
			relPath := strings.TrimPrefix(*obj.Key, prefix)
			relPath = strings.TrimPrefix(relPath, "/")
			if relPath == "" {
				continue
			}

			localPath := filepath.Join(tmpDir, relPath)
			if err := downloadS3Object(ctx, client, bucket, *obj.Key, localPath); err != nil {
				return err
			}
		}
	}

	return nil
}

func resolveGitPath(ctx context.Context, fsc *FileSourceConfig, path string) (string, func(), error) {
	noop := func() {}

	var repoURL, subPath, ref string
	var token, sshKeyPath string

	if fsc != nil && fsc.GitSource != nil {
		repoURL = fsc.GitSource.URL
		subPath = fsc.GitSource.Path
		ref = fsc.GitSource.Ref
		token = fsc.GitSource.Token
		sshKeyPath = fsc.GitSource.SSHKeyPath
	}

	if strings.HasPrefix(path, "git::") {
		parsedURL, parsedSubPath, parsedRef := ParseGitPath(path)
		if repoURL == "" {
			repoURL = parsedURL
		}
		if subPath == "" {
			subPath = parsedSubPath
		}
		if ref == "" {
			ref = parsedRef
		}
	}

	if repoURL == "" {
		return "", noop, fmt.Errorf("Git repository URL not specified")
	}

	if ref == "" {
		ref = "main"
	}

	tmpDir, err := os.MkdirTemp("", "marmot-git-*")
	if err != nil {
		return "", noop, fmt.Errorf("creating temp directory: %w", err)
	}
	cleanup := func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			log.Warn().Err(err).Str("dir", tmpDir).Msg("Failed to clean up temp directory")
		}
	}

	cloneOpts := &git.CloneOptions{
		URL:   repoURL,
		Depth: 1,
	}

	if token != "" {
		cloneOpts.Auth = &githttp.BasicAuth{
			Username: "x-access-token",
			Password: token,
		}
	} else if sshKeyPath != "" {
		keys, err := gitssh.NewPublicKeysFromFile("git", sshKeyPath, "")
		if err != nil {
			cleanup()
			return "", noop, fmt.Errorf("loading SSH key from %s: %w", sshKeyPath, err)
		}
		cloneOpts.Auth = keys
	}

	cloneOpts.ReferenceName = plumbing.NewBranchReferenceName(ref)
	repo, err := git.PlainCloneContext(ctx, tmpDir, false, cloneOpts)
	if err != nil {
		if err2 := os.RemoveAll(tmpDir); err2 != nil {
			log.Warn().Err(err2).Msg("Failed to clean up failed clone dir")
		}
		if err3 := os.MkdirAll(tmpDir, 0o750); err3 != nil {
			cleanup()
			return "", noop, fmt.Errorf("re-creating temp dir: %w", err3)
		}

		cloneOpts.ReferenceName = plumbing.NewTagReferenceName(ref)
		repo, err = git.PlainCloneContext(ctx, tmpDir, false, cloneOpts)
		if err != nil {
			cleanup()
			return "", noop, fmt.Errorf("cloning repository %s (ref %s): %w", repoURL, ref, err)
		}
	}

	_ = repo

	localPath := tmpDir
	if subPath != "" {
		localPath = filepath.Join(tmpDir, subPath)
	}

	return localPath, cleanup, nil
}
