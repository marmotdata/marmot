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

	"github.com/spf13/cobra"

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

var (
	serve               bool
	serveCAFingerprints []string
	servePolicyFile     string
	serveAuditLogFile   string
	serveAllowActions   bool
)

// serveFlagsHelp documents the serving flags and the policy file, in the help of the command that takes them.
const serveFlagsHelp = `With --serve, the process stays running after ingesting and answers Marmot's queries for the run marked queryable: true.

  --ca-fingerprint sha256:<hex>   Check Marmot's serving CA before first trusting it. Repeatable;
                                  copy the values from 'marmot admin serve-keys'.
  --policy <file.yaml>            This process's own limits on what Marmot may run here.
  --audit-log <path>              Append a hash-chained JSON line per query served or refused;
                                  check it with 'marmot ingest verify-audit-log <path>'.
  --allow-actions                 Also serve plugin actions and run triggers, each only if the policy
                                  names it. Needs --audit-log: an action's intent is written there
                                  before it runs.

A policy file names each source this process may serve. Once a policy is given, a source it does
not name is refused, and unknown keys are an error:

  sources:
    warehouse:
      query: true            # default true; false refuses every plan and execute
      max_rows: 5000         # optional ceiling; a signed row limit above it is lowered to it
      actors: ["user:alice", "task:*"]   # optional allow-list of the signed actor; * matches anything
      origins: ["task_run:*"]            # optional allow-list of what caused the query or action

Actions are refused unless --allow-actions is given and the policy names each one. Rules on an
action only narrow what the plugin's manifest allows; a constrained parameter must be given:

  sources:
    prod-cluster:
      query: false
      manifest: sha256:<hex>   # optional; refuse actions if the plugin's manifest changes
      actions:
        rollout_restart:
          origins: ["task_run:*"]
        scale:
          approvals: 1           # people who must approve each execute; default 1, at most 2
          params:
            namespace: {values: [payments, checkout]}
            replicas: {min: 1, max: 10}
        workload_status: {}    # a read: never needs approval
      triggers:                # only these run, with these parameters; Marmot only receives events
        workload_unhealthy:
          params: {kinds: [deployment], namespaces: [payments], conditions: [crash_looping]}
          cooldown: 30m

A write runs only with an approval Marmot signed with its approval key, pinned
from enrolment like the query signing keys, and checked here: the signature,
the expiry and enough distinct approvers. Approvals are Marmot-signed, which
this process logs at start and states on every receipt:

  approvals:
    trust: marmot          # the only mode today; oidc is planned
    max_age: 15m
    key_fingerprint: sha256:<from marmot admin serve-keys>   # optional`

// serveSettings reads the serving flags before anything is ingested, so a bad fingerprint, policy or audit log path fails at once rather than after a long ingestion.
type serveSettings struct {
	fingerprints []string
	policy       *pluginserve.Policy
	auditLog     *pluginserve.AuditLog
}

func loadServeSettings() (*serveSettings, error) {
	settings := &serveSettings{}
	for _, raw := range serveCAFingerprints {
		fp, err := pluginserve.ParseCAFingerprint(raw)
		if err != nil {
			return nil, err
		}
		settings.fingerprints = append(settings.fingerprints, fp)
	}
	if servePolicyFile != "" {
		policy, err := pluginserve.LoadPolicy(servePolicyFile)
		if err != nil {
			return nil, err
		}
		settings.policy = policy
	}
	if serveAllowActions && serveAuditLogFile == "" {
		return nil, errors.New("--allow-actions needs --audit-log: no action runs unless its intent is written down first")
	}
	if serveAuditLogFile != "" {
		auditLog, err := pluginserve.OpenAuditLog(serveAuditLogFile)
		if err != nil {
			return nil, err
		}
		settings.auditLog = auditLog
	}
	return settings, nil
}

// serveOnlyFlagsSet reports a serving flag given without --serve, which would otherwise be silently ignored.
func serveOnlyFlagsSet() bool {
	return len(serveCAFingerprints) > 0 || servePolicyFile != "" || serveAuditLogFile != "" || serveAllowActions
}

var verifyAuditLogCmd = &cobra.Command{
	Use:   "verify-audit-log <path>",
	Short: "Check that a serving process's audit log has not been changed",
	Long:  `Checks that every line of an audit log written with --audit-log follows from the one before it. A changed or removed line breaks the chain at the line after it, which is reported. Lines removed from the end leave no break; compare the log with the receipts Marmot records for each query to catch those.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		n, err := pluginserve.VerifyAuditLog(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("%d lines verified\n", n)
		return nil
	},
}

func init() {
	ingestCmd.Flags().StringArrayVar(&serveCAFingerprints, "ca-fingerprint", nil, "With --serve, a SHA-256 fingerprint (sha256:<hex>) of a CA Marmot's first enrolment must include; repeatable")
	ingestCmd.Flags().StringVar(&servePolicyFile, "policy", "", "With --serve, a YAML policy limiting what this process serves")
	ingestCmd.Flags().StringVar(&serveAuditLogFile, "audit-log", "", "With --serve, append a hash-chained JSON line per query to this file")
	ingestCmd.Flags().BoolVar(&serveAllowActions, "allow-actions", false, "With --serve, also serve the plugin actions the policy names; needs --audit-log")
	ingestCmd.Long = serveFlagsHelp
	ingestCmd.AddCommand(verifyAuditLogCmd)
}

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
		// A plugin with actions and no queries, such as Kubernetes, is worth serving only when this process may run actions
		if !entry.Meta.SupportsQuery && !(serveAllowActions && entry.Meta.SupportsActions) {
			if entry.Meta.SupportsActions {
				return nil, fmt.Errorf("plugin %s does not support queries; it has actions, which it serves only with --allow-actions", pluginID)
			}
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
func serveQueries(ctx context.Context, marmotURL *url.URL, sources []pluginserve.LocalSource, settings *serveSettings) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if settings.auditLog != nil {
		defer func() { _ = settings.auditLog.Close() }()
	}

	instances := plugin.NewInstanceManager(serveInstanceIdleTTL)
	defer instances.Stop()

	// The action host is built only when actions are switched on, and only here: this serving process is the one place a plugin's actions may run.
	var actor pluginserve.Actor
	var watcher pluginserve.TriggerWatcher
	if serveAllowActions {
		host := plugin.NewActionHost(instances)
		actor, watcher = host, host
	}

	printStep(fmt.Sprintf("Serving queries for %s", sources[0].Name))
	return pluginserve.Serve(ctx, pluginserve.ServeOptions{
		Host:    marmotURL.Hostname(),
		Sources: sources,
		Enrol:   enrolFunc(marmotURL.String()),
		Querier: instances,

		CAFingerprints: settings.fingerprints,
		Policy:         settings.policy,
		AuditLog:       settings.auditLog,
		AllowActions:   serveAllowActions,
		Actor:          actor,
		Watcher:        watcher,
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
