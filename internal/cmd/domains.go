package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/marmotdata/marmot/internal/cmd/output"
	"github.com/spf13/cobra"
)

// Fork-only: domains are not in the generated SDK, so these commands call the
// API directly with the same host and credentials as the rest of the CLI.

type domainRef struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

type enforcementState struct {
	Write     bool    `json:"write"`
	UpdatedBy *string `json:"updated_by"`
	UpdatedAt *string `json:"updated_at"`
}

type enforcementPlan struct {
	State      enforcementState `json:"state"`
	Principals []struct {
		SubjectType string      `json:"subject_type"`
		Name        string      `json:"name"`
		Permissions []string    `json:"permissions"`
		Keeps       []domainRef `json:"keeps"`
		Loses       []domainRef `json:"loses"`
	} `json:"principals"`
	Pipelines []struct {
		Name          string    `json:"name"`
		Domain        domainRef `json:"domain"`
		AssetsOutside int       `json:"assets_outside"`
	} `json:"pipelines"`
	Hash string `json:"hash"`
}

func domainsAPI(ctx context.Context, method, path string, body, out any) error {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(getHost(), "/")+"/api/v1/domains"+path, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "marmot-cli")
	if token, bearer := getAuthToken(); bearer {
		req.Header.Set("Authorization", "Bearer "+token)
	} else if token != "" {
		req.Header.Set("X-API-Key", token)
	}
	resp, err := http.DefaultClient.Do(req) //nolint:gosec // G704: the host is the operator's configured Marmot server
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		var e struct {
			Error string `json:"error"`
			Code  string `json:"code"`
		}
		if json.Unmarshal(data, &e) == nil && e.Error != "" {
			return fmt.Errorf("%s (%s)", e.Error, e.Code)
		}
		return fmt.Errorf("request failed with status %d", resp.StatusCode)
	}
	return json.Unmarshal(data, out)
}

func printEnforcementState(s enforcementState) {
	p := getPrinter()
	t := output.NewTable("FIELD", "VALUE")
	state := "off"
	if s.Write {
		state = "on"
	}
	t.AddRow("Write enforcement", state)
	if s.UpdatedBy != nil {
		t.AddRow("Changed by", *s.UpdatedBy)
	}
	if s.UpdatedAt != nil {
		t.AddRow("Changed at", *s.UpdatedAt)
	}
	p.PrintTable(t)
}

func paths(refs []domainRef) string {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		out = append(out, r.Path)
	}
	if len(out) == 0 {
		return "-"
	}
	return strings.Join(out, ", ")
}

var domainsCmd = &cobra.Command{
	Use:   "domains",
	Short: "Manage domains",
}

var domainsEnforcementCmd = &cobra.Command{
	Use:   "enforcement",
	Short: "Review and switch domain-scoped write enforcement",
}

var domainsEnforcementStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show whether writes are scoped by domain",
	RunE: func(cmd *cobra.Command, _ []string) error {
		var s enforcementState
		if err := domainsAPI(cmd.Context(), http.MethodGet, "/enforcement", nil, &s); err != nil {
			return err
		}
		if getPrinter().IsRaw() {
			return printRaw(s)
		}
		printEnforcementState(s)
		return nil
	},
}

var domainsEnforcementPlanCmd = &cobra.Command{
	Use:   "plan",
	Short: "List who would lose write access, and where, once enforcement is on",
	RunE: func(cmd *cobra.Command, _ []string) error {
		var plan enforcementPlan
		if err := domainsAPI(cmd.Context(), http.MethodGet, "/enforcement/plan", nil, &plan); err != nil {
			return err
		}
		p := getPrinter()
		if p.IsRaw() {
			return printRaw(plan)
		}
		t := output.NewTable("TYPE", "NAME", "PERMISSIONS", "KEEPS", "LOSES")
		for _, pr := range plan.Principals {
			t.AddRow(pr.SubjectType, pr.Name, strings.Join(pr.Permissions, ","), paths(pr.Keeps), paths(pr.Loses))
		}
		t.SetFooter("%d identities lose write access somewhere", len(plan.Principals))
		p.PrintTable(t)
		if len(plan.Pipelines) > 0 {
			pt := output.NewTable("PIPELINE", "DOMAIN", "ASSETS OUTSIDE")
			for _, pl := range plan.Pipelines {
				pt.AddRow(pl.Name, pl.Domain.Path, fmt.Sprint(pl.AssetsOutside))
			}
			pt.SetFooter("Runs of these pipelines can no longer update or remove those assets")
			p.PrintTable(pt)
		}
		p.PrintMessage("Plan %s. To activate it: marmot domains enforcement enable --confirm %s", plan.Hash, plan.Hash)
		return nil
	},
}

var domainsEnforcementEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Scope writes by domain, confirming the plan you reviewed",
	RunE: func(cmd *cobra.Command, _ []string) error {
		confirm, _ := cmd.Flags().GetString("confirm")
		if confirm == "" {
			return fmt.Errorf("--confirm is required: run 'marmot domains enforcement plan' and pass its hash")
		}
		return switchEnforcement(cmd, true, confirm)
	},
}

var domainsEnforcementDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Stop scoping writes by domain",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return switchEnforcement(cmd, false, "")
	},
}

func switchEnforcement(cmd *cobra.Command, on bool, confirm string) error {
	var s enforcementState
	body := map[string]any{"write": on, "confirm": confirm}
	if err := domainsAPI(cmd.Context(), http.MethodPost, "/enforcement", body, &s); err != nil {
		return err
	}
	if getPrinter().IsRaw() {
		return printRaw(s)
	}
	printEnforcementState(s)
	return nil
}

func printRaw(v any) error {
	data, err := marshalPayload(v)
	if err != nil {
		return err
	}
	return getPrinter().PrintRaw(data)
}

func init() {
	domainsEnforcementEnableCmd.Flags().String("confirm", "", "Hash of the reviewed plan")
	domainsEnforcementCmd.AddCommand(domainsEnforcementStatusCmd, domainsEnforcementPlanCmd, domainsEnforcementEnableCmd, domainsEnforcementDisableCmd)
	domainsCmd.AddCommand(domainsEnforcementCmd)
	rootCmd.AddCommand(domainsCmd)
}
