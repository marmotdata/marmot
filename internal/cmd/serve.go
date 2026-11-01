package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os/signal"
	"syscall"
	"time"

	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/marmotdata/marmot/internal/pluginserve"
)

const (
	apiServeEnrol = "/api/v1/query/serve/enrol"
	// queryableKey marks a run whose plugin answers queries under --serve. It is Marmot's setting, so it is removed before the config reaches the plugin.
	queryableKey = "queryable"
	// serveInstanceIdleTTL is how long an unused local plugin process is kept between queries.
	serveInstanceIdleTTL = 5 * time.Minute
)

var serve bool

// takeQueryableRuns removes the queryable flag from every run and returns the plugin IDs and configs of the runs that set it.
func takeQueryableRuns(runs []plugin.SourceRun) (map[string]plugin.RawPluginConfig, error) {
	queryable := make(map[string]plugin.RawPluginConfig)
	for _, run := range runs {
		for pluginID, config := range run {
			flag, present := config[queryableKey]
			delete(config, queryableKey)
			if !present {
				continue
			}
			on, ok := flag.(bool)
			if !ok {
				return nil, fmt.Errorf("%s: %s must be true or false", pluginID, queryableKey)
			}
			if on {
				queryable[pluginID] = config
			}
		}
	}
	return queryable, nil
}

// servedSources turns the pipeline's queryable runs into the sources this process serves. A served source takes the pipeline's name, which is how it is queried, so a pipeline serves at most one source.
func servedSources(pipeline string, queryable map[string]plugin.RawPluginConfig) ([]pluginserve.LocalSource, error) {
	if len(queryable) == 0 {
		return nil, errors.New("--serve needs a run with queryable: true")
	}
	if len(queryable) > 1 {
		return nil, errors.New("a pipeline can serve one queryable run; split the others into their own pipelines")
	}

	var sources []pluginserve.LocalSource
	for pluginID, config := range queryable {
		entry, err := plugin.GetRegistry().Get(pluginID)
		if err != nil {
			return nil, fmt.Errorf("unknown source: %s", pluginID)
		}
		if !entry.Meta.SupportsQuery {
			return nil, fmt.Errorf("plugin %s does not support queries", pluginID)
		}
		if _, err := entry.Source.Validate(config); err != nil {
			return nil, fmt.Errorf("%s: config validation failed: %w", pluginID, err)
		}
		sources = append(sources, pluginserve.LocalSource{
			Name:          pipeline,
			PluginID:      pluginID,
			Config:        config,
			DisplayConfig: plugin.MaskSensitiveFieldsFromSpec(config, entry.Meta.ConfigSpec),
		})
	}
	return sources, nil
}

// serveQueries keeps this process connected to Marmot, answering queries for sources from local plugin processes, until it is interrupted.
func serveQueries(ctx context.Context, marmotURL *url.URL, sources []pluginserve.LocalSource) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	instances := plugin.NewInstanceManager(serveInstanceIdleTTL)
	defer instances.Stop()

	printStep(fmt.Sprintf("Serving queries for %s", sources[0].Name))
	return pluginserve.Serve(ctx, pluginserve.ServeOptions{
		Host:    marmotURL.Hostname(),
		Sources: sources,
		Enrol:   enrolFunc(marmotURL.String()),
		Querier: instances,
	})
}

// enrolmentURL checks host is safe to enrol over. The enrolment request carries this process's Marmot credential, and the response carries the CAs and signing keys it will trust, so it must travel over HTTPS. Plain HTTP is refused even to localhost: on a shared machine another user can listen on a loopback port first. Use --ca-file for a Marmot behind a private CA.
func enrolmentURL(host string) (*url.URL, error) {
	u, err := url.Parse(host)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("invalid Marmot host %q", host)
	}
	if u.Scheme != "https" {
		return nil, fmt.Errorf("--serve needs an https Marmot host, because it sends its credential to it and learns what to trust from it")
	}
	return u, nil
}

// enrolFunc enrols over the API, resolving the credential afresh each time so a rotated Kubernetes ServiceAccount token is picked up.
func enrolFunc(baseURL string) pluginserve.EnrolFunc {
	return func(ctx context.Context, in pluginserve.EnrolInput) (*pluginserve.EnrolResult, error) {
		token, isBearer := getAuthToken()
		client, err := newAPIClient(baseURL, token, isBearer)
		if err != nil {
			return nil, err
		}
		req, err := client.newRequest(ctx, http.MethodPost, apiServeEnrol, in)
		if err != nil {
			return nil, err
		}
		var res pluginserve.EnrolResult
		if err := client.do(req, &res); err != nil {
			return nil, err
		}
		return &res, nil
	}
}
