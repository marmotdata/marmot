package cloudrun

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/api/option"
	run "google.golang.org/api/run/v2"
)

// maxPages bounds every list loop. A project with more pages than this is far
// outside anything Cloud Run's quotas allow, so hitting the cap means
// something is wrong and the run should say so rather than spin forever.
const maxPages = 200

// pageSize is the largest page Cloud Run accepts, so a project is walked in as
// few round trips as possible.
const pageSize = 1000

// requestTimeout bounds a single page request, so one region that stops
// answering cannot use up the whole discovery budget.
const requestTimeout = 30 * time.Second

// client wraps the generated Cloud Run Admin API client with the pagination
// every list call in this plugin needs.
type client struct {
	svc *run.Service
}

// newClient builds a Cloud Run Admin API client. The credential options are
// tried in a fixed order so a config that sets more than one is predictable:
// no authentication at all beats an inline key, which beats a key file. With
// none of them set the Google client library falls back to the credentials
// the environment already provides.
func newClient(ctx context.Context, config *Config) (*client, error) {
	var opts []option.ClientOption

	if config.Endpoint != "" {
		opts = append(opts, option.WithEndpoint(config.Endpoint))
	}

	switch {
	case config.DisableAuth:
		opts = append(opts, option.WithoutAuthentication())
	case config.CredentialsJSON != "":
		opts = append(opts, option.WithCredentialsJSON([]byte(config.CredentialsJSON)))
	case config.CredentialsFile != "":
		opts = append(opts, option.WithCredentialsFile(config.CredentialsFile))
	}

	svc, err := run.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating Cloud Run client: %w", err)
	}

	return &client{svc: svc}, nil
}

// listServices returns every service under parent, following page tokens to
// the end. Cloud Run reports regions it could not reach in the same response
// instead of failing, so those names are returned alongside the services for
// the caller to warn about.
func (c *client) listServices(ctx context.Context, parent string) ([]*run.GoogleCloudRunV2Service, []string, error) {
	var (
		services    []*run.GoogleCloudRunV2Service
		unreachable []string
		token       string
	)

	for page := 0; page < maxPages; page++ {
		pageCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		resp, err := c.svc.Projects.Locations.Services.List(parent).
			PageSize(pageSize).
			PageToken(token).
			Context(pageCtx).
			Do()
		cancel()
		if err != nil {
			return nil, nil, fmt.Errorf("listing services under %s: %w", parent, err)
		}

		services = append(services, resp.Services...)
		unreachable = append(unreachable, resp.Unreachable...)

		token = resp.NextPageToken
		if token == "" {
			return services, unreachable, nil
		}
	}

	log.Warn().Str("parent", parent).Int("max_pages", maxPages).Msg("Stopped listing services at the page limit")
	return services, unreachable, nil
}

// listJobs returns every job under parent, following page tokens to the end.
func (c *client) listJobs(ctx context.Context, parent string) ([]*run.GoogleCloudRunV2Job, error) {
	var (
		jobs  []*run.GoogleCloudRunV2Job
		token string
	)

	for page := 0; page < maxPages; page++ {
		pageCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		resp, err := c.svc.Projects.Locations.Jobs.List(parent).
			PageSize(pageSize).
			PageToken(token).
			Context(pageCtx).
			Do()
		cancel()
		if err != nil {
			return nil, fmt.Errorf("listing jobs under %s: %w", parent, err)
		}

		jobs = append(jobs, resp.Jobs...)

		token = resp.NextPageToken
		if token == "" {
			return jobs, nil
		}
	}

	log.Warn().Str("parent", parent).Int("max_pages", maxPages).Msg("Stopped listing jobs at the page limit")
	return jobs, nil
}

// listExecutions returns at most limit executions of one job. Executions come
// back newest first, so stopping early keeps the most recent runs.
func (c *client) listExecutions(ctx context.Context, jobName string, limit int) ([]*run.GoogleCloudRunV2Execution, error) {
	var (
		executions []*run.GoogleCloudRunV2Execution
		token      string
	)

	for page := 0; page < maxPages; page++ {
		pageCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		resp, err := c.svc.Projects.Locations.Jobs.Executions.List(jobName).
			PageSize(int64(limit)).
			PageToken(token).
			Context(pageCtx).
			Do()
		cancel()
		if err != nil {
			return nil, fmt.Errorf("listing executions of %s: %w", jobName, err)
		}

		executions = append(executions, resp.Executions...)
		if len(executions) >= limit {
			return executions[:limit], nil
		}

		token = resp.NextPageToken
		if token == "" {
			return executions, nil
		}
	}

	log.Warn().Str("job", jobName).Int("max_pages", maxPages).Msg("Stopped listing executions at the page limit")
	return executions, nil
}

// parents returns the list parents to scan: one per configured region, or the
// single "-" wildcard parent that Cloud Run expands to every region.
func (s *Source) parents() []string {
	if len(s.config.Locations) == 0 {
		return []string{fmt.Sprintf("projects/%s/locations/-", s.config.ProjectID)}
	}

	parents := make([]string, 0, len(s.config.Locations))
	for _, location := range s.config.Locations {
		parents = append(parents, fmt.Sprintf("projects/%s/locations/%s", s.config.ProjectID, location))
	}
	return parents
}

// resourceName is the identifying triple inside a fully qualified Cloud Run
// resource name such as projects/acme/locations/europe-west1/services/checkout-api.
type resourceName struct {
	Project  string
	Location string
	ID       string
}

// parseResourceName splits a service or job resource name. The collection
// segment ("services" or "jobs") is not returned because the caller already
// knows which list it asked for.
func parseResourceName(name string) (resourceName, error) {
	parts := strings.Split(name, "/")
	if len(parts) != 6 || parts[0] != "projects" || parts[2] != "locations" {
		return resourceName{}, fmt.Errorf("unexpected Cloud Run resource name %q", name)
	}
	if parts[1] == "" || parts[3] == "" || parts[5] == "" {
		return resourceName{}, fmt.Errorf("unexpected Cloud Run resource name %q", name)
	}

	return resourceName{Project: parts[1], Location: parts[3], ID: parts[5]}, nil
}

// resourceID returns the last segment of a resource name, which is the bare id
// users see in the console and the CLI. Revisions and executions are only ever
// referenced by that id, so their full names never reach the catalog.
func resourceID(name string) string {
	if i := strings.LastIndex(name, "/"); i >= 0 {
		return name[i+1:]
	}
	return name
}
