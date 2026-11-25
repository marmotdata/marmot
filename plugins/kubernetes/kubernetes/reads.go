package kubernetes

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
)

// The read verbs let a task see the cluster before it proposes a change.
// They are tools of risk 0 so the runner gates them exactly like writes
// (named in its policy, scoped by tools.namespaces and tools.selector,
// audited) but they never need a plan, a precondition or an approval.
// GitOps-managed objects are readable: reading one cannot fight Git.

// Read tool names.
const (
	toolWorkloadStatus = "workload_status"
	toolPodLogs        = "pod_logs"
	toolRecentEvents   = "recent_events"
)

const (
	kindPod       pluginsdk.TargetKind = "kubernetes/pod"
	kindNamespace pluginsdk.TargetKind = "kubernetes/namespace"
)

const (
	// maxStatusPods bounds the pods workload_status describes.
	maxStatusPods = 20
	// maxLogLine caps each log line after redaction, in bytes.
	maxLogLine = 2 << 10
	// maxLogScan is how much of one line is read and redacted before the
	// rest is discarded, so a secret that straddles the 2 KiB cut is
	// still seen whole by the redactor.
	maxLogScan = 8 << 10
	// maxLogBytes bounds the lines pod_logs returns, well under the SDK's
	// cap on a read, so the JSON around them always fits.
	maxLogBytes = 128 << 10
	// maxLogFetch bounds what is read from the API server at all.
	maxLogFetch = 512 << 10
	// maxEventMessage caps an event message, which anything able to create
	// events in the namespace can write.
	maxEventMessage = 512
	// maxConditionMessage caps a workload condition's message.
	maxConditionMessage = 256
)

func readSpecs() []pluginsdk.ToolSpec {
	return []pluginsdk.ToolSpec{
		{
			Name:        toolWorkloadStatus,
			Version:     1,
			Title:       "Read a workload's status",
			Description: "Returns a Deployment, StatefulSet or DaemonSet's replica counts and conditions, and up to 20 of its pods with their phase, readiness, restarts, waiting reasons and last termination.",
			Targets:     []pluginsdk.TargetKind{kindDeployment, kindStatefulSet, kindDaemonSet},
			Params: []pluginsdk.ToolParam{
				{Name: "kind", Description: "Kind of workload", Type: pluginsdk.ParamEnum, Required: true, Enum: []string{"deployment", "statefulset", "daemonset"}},
				namespaceParam, nameParam,
			},
			Reversibility: pluginsdk.ReversibilityReadOnly,
			Risk:          pluginsdk.RiskRead,
		},
		{
			Name:        toolPodLogs,
			Version:     1,
			Title:       "Read a pod's logs",
			Description: "Returns the last lines a container logged, with credentials redacted and each line capped at 2 KiB. Log text is written by the workload, so it is marked untrusted.",
			Targets:     []pluginsdk.TargetKind{kindPod},
			Params: []pluginsdk.ToolParam{
				namespaceParam,
				{Name: "pod", Description: "Name of the pod", Type: pluginsdk.ParamString, Required: true, MaxLength: 253, Pattern: dns1123Subdomain},
				{Name: "container", Description: "Container to read. Defaults to the pod's first container", Type: pluginsdk.ParamString, MaxLength: 63, Pattern: dns1123Label},
				{Name: "tail_lines", Description: "How many of the last lines to return", Type: pluginsdk.ParamInt, Required: true, Min: 1, Max: 200},
				{Name: "previous", Description: "Read the previous, terminated instance of the container, as after a crash", Type: pluginsdk.ParamBool},
			},
			Reversibility: pluginsdk.ReversibilityReadOnly,
			Risk:          pluginsdk.RiskRead,
		},
		{
			Name:        toolRecentEvents,
			Version:     1,
			Title:       "Read recent events",
			Description: "Returns a namespace's most recent events, optionally for one object, newest first, with messages capped at 512 characters. Event messages are free text, so they are marked untrusted.",
			Targets:     []pluginsdk.TargetKind{kindNamespace},
			Params: []pluginsdk.ToolParam{
				namespaceParam,
				{Name: "involved_kind", Description: "Kind of the object the events are about", Type: pluginsdk.ParamEnum, Enum: []string{"pod", "deployment", "statefulset", "daemonset", "job", "cronjob"}},
				{Name: "involved_name", Description: "Name of the object the events are about; needs involved_kind", Type: pluginsdk.ParamString, MaxLength: 253, Pattern: dns1123Subdomain},
				{Name: "limit", Description: "How many events to return", Type: pluginsdk.ParamInt, Required: true, Min: 1, Max: 50},
			},
			Reversibility: pluginsdk.ReversibilityReadOnly,
			Risk:          pluginsdk.RiskRead,
		},
	}
}

func isRead(tool string) bool {
	return tool == toolWorkloadStatus || tool == toolPodLogs || tool == toolRecentEvents
}

// read runs a read verb. The request is checked against the manifest here
// as well as in Serve, since the engine is exported for the cloud plugins.
func (e *ToolEngine) read(ctx context.Context, req pluginsdk.ToolRequest) (*pluginsdk.ToolResult, error) {
	if _, err := pluginsdk.CheckToolRequest(ToolSpecs(), req, true); err != nil {
		return nil, err
	}
	ns := req.StringParam("namespace")
	if !e.config.inScope(ns) {
		return nil, fmt.Errorf("namespace %q is %w: allow it in tools.namespaces", ns, ErrOutOfScope)
	}
	var (
		target  string
		kind    pluginsdk.TargetKind
		summary string
		out     any
		err     error
	)
	switch req.Tool {
	case toolWorkloadStatus:
		var s *workloadStatus
		s, err = e.workloadStatus(ctx, req)
		if s != nil {
			target, kind, out = s.Kind+"/"+ns+"/"+s.Name, pluginsdk.TargetKind("kubernetes/"+s.Kind), s
			summary = fmt.Sprintf("%s: %d of %d replicas ready", target, s.Replicas.Ready, s.Replicas.Desired)
		}
	case toolPodLogs:
		var l *podLogs
		l, err = e.podLogs(ctx, req)
		if l != nil {
			target, kind, out = "pod/"+ns+"/"+l.Pod, kindPod, l
			summary = fmt.Sprintf("%d lines from %s container %s", len(l.Lines), target, l.Container)
		}
	case toolRecentEvents:
		var ev *recentEvents
		ev, err = e.recentEvents(ctx, req)
		if ev != nil {
			target, kind, out = "namespace/"+ns, kindNamespace, ev
			summary = fmt.Sprintf("%d recent events in %s", len(ev.Events), ns)
		}
	default:
		err = fmt.Errorf("unknown read %q", req.Tool)
	}
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("encoding the read: %w", err)
	}
	// Every read carries text the cluster's users can write: condition and
	// waiting messages, log lines, event messages.
	return &pluginsdk.ToolResult{Target: target, TargetKind: kind, Summary: summary, Output: data, Untrusted: true}, nil
}

type replicaCounts struct {
	Desired     int32 `json:"desired"`
	Ready       int32 `json:"ready"`
	Updated     int32 `json:"updated"`
	Available   int32 `json:"available"`
	Unavailable int32 `json:"unavailable"`
}

type workloadCondition struct {
	Type           string     `json:"type"`
	Status         string     `json:"status"`
	Reason         string     `json:"reason,omitempty"`
	Message        string     `json:"message,omitempty"`
	LastTransition *time.Time `json:"last_transition,omitempty"`
}

type containerStatus struct {
	Name          string `json:"name"`
	Ready         bool   `json:"ready"`
	RestartCount  int32  `json:"restart_count"`
	State         string `json:"state"`
	WaitingReason string `json:"waiting_reason,omitempty"`
	// LastTerminationReason and LastExitCode describe the container's
	// last finished run, such as the crash before a CrashLoopBackOff.
	LastTerminationReason string     `json:"last_termination_reason,omitempty"`
	LastExitCode          *int32     `json:"last_exit_code,omitempty"`
	LastFinishedAt        *time.Time `json:"last_finished_at,omitempty"`
	// FailingSince is set for a container that is failing now. Kubernetes
	// keeps only a container's last termination, so for one that has
	// restarted it is the pod's start time, the earliest it can have begun
	// failing; for one that has not, it is that last termination's finish.
	// It is therefore an upper bound on how long the failure has lasted.
	FailingSince *time.Time `json:"failing_since,omitempty"`
}

type podStatus struct {
	Name              string            `json:"name"`
	Phase             string            `json:"phase"`
	Ready             bool              `json:"ready"`
	Node              string            `json:"node,omitempty"`
	StartedAt         *time.Time        `json:"started_at,omitempty"`
	Restarts          int32             `json:"restarts"`
	FailingSince      *time.Time        `json:"failing_since,omitempty"`
	FailingForSeconds int64             `json:"failing_for_seconds,omitempty"`
	Containers        []containerStatus `json:"containers"`
}

type workloadStatus struct {
	Kind          string              `json:"kind"`
	Namespace     string              `json:"namespace"`
	Name          string              `json:"name"`
	Replicas      replicaCounts       `json:"replicas"`
	Conditions    []workloadCondition `json:"conditions,omitempty"`
	Pods          []podStatus         `json:"pods"`
	PodsTotal     int                 `json:"pods_total"`
	PodsTruncated bool                `json:"pods_truncated,omitempty"`
	Untrusted     bool                `json:"untrusted"`
}

func (e *ToolEngine) workloadStatus(ctx context.Context, req pluginsdk.ToolRequest) (*workloadStatus, error) {
	ns, name, kind := req.StringParam("namespace"), req.StringParam("name"), req.StringParam("kind")
	out := &workloadStatus{Kind: kind, Namespace: ns, Name: name, Untrusted: true}
	var obj metav1.Object
	var selector *metav1.LabelSelector
	switch kind {
	case "deployment":
		d, err := e.client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("reading deployment/%s/%s: %w", ns, name, err)
		}
		obj, selector = d, d.Spec.Selector
		out.Replicas = replicaCounts{Desired: replicasOf(d.Spec.Replicas), Ready: d.Status.ReadyReplicas, Updated: d.Status.UpdatedReplicas, Available: d.Status.AvailableReplicas, Unavailable: d.Status.UnavailableReplicas}
		for _, c := range d.Status.Conditions {
			out.Conditions = append(out.Conditions, condition(string(c.Type), string(c.Status), c.Reason, c.Message, c.LastTransitionTime))
		}
	case "statefulset":
		s, err := e.client.AppsV1().StatefulSets(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("reading statefulset/%s/%s: %w", ns, name, err)
		}
		obj, selector = s, s.Spec.Selector
		desired := replicasOf(s.Spec.Replicas)
		out.Replicas = replicaCounts{Desired: desired, Ready: s.Status.ReadyReplicas, Updated: s.Status.UpdatedReplicas, Available: s.Status.AvailableReplicas, Unavailable: max(desired-s.Status.AvailableReplicas, 0)}
		for _, c := range s.Status.Conditions {
			out.Conditions = append(out.Conditions, condition(string(c.Type), string(c.Status), c.Reason, c.Message, c.LastTransitionTime))
		}
	case "daemonset":
		d, err := e.client.AppsV1().DaemonSets(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("reading daemonset/%s/%s: %w", ns, name, err)
		}
		obj, selector = d, d.Spec.Selector
		out.Replicas = replicaCounts{Desired: d.Status.DesiredNumberScheduled, Ready: d.Status.NumberReady, Updated: d.Status.UpdatedNumberScheduled, Available: d.Status.NumberAvailable, Unavailable: d.Status.NumberUnavailable}
		for _, c := range d.Status.Conditions {
			out.Conditions = append(out.Conditions, condition(string(c.Type), string(c.Status), c.Reason, c.Message, c.LastTransitionTime))
		}
	default:
		return nil, fmt.Errorf("workload_status does not read kind %q", kind)
	}
	if !e.selector.Matches(labels.Set(obj.GetLabels())) {
		return nil, fmt.Errorf("%s/%s/%s does not match tools.selector %q: %w", kind, ns, name, e.config.Selector, ErrOutOfScope)
	}

	sel, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil || sel.Empty() {
		// An empty selector would list every pod in the namespace.
		return nil, fmt.Errorf("%s/%s/%s has no usable pod selector", kind, ns, name)
	}
	pods, err := e.client.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: sel.String()})
	if err != nil {
		return nil, fmt.Errorf("listing the pods of %s/%s/%s: %w", kind, ns, name, err)
	}
	items := pods.Items
	// Failing pods first, so the 20 kept are the ones worth reading.
	now := e.now()
	described := make([]podStatus, 0, len(items))
	for i := range items {
		if ref := metav1.GetControllerOf(&items[i]); ref != nil && !ownedBy(ref, kind, name, obj) {
			continue
		}
		described = append(described, describePod(&items[i], now))
	}
	sort.SliceStable(described, func(i, j int) bool {
		if (described[i].FailingSince != nil) != (described[j].FailingSince != nil) {
			return described[i].FailingSince != nil
		}
		return described[i].Name < described[j].Name
	})
	out.PodsTotal = len(described)
	if len(described) > maxStatusPods {
		described, out.PodsTruncated = described[:maxStatusPods], true
	}
	out.Pods = described
	return out, nil
}

// ownedBy reports whether a pod's controller is the workload, directly or,
// for a Deployment, through one of its ReplicaSets, named
// <deployment>-<pod-template-hash>, a hash with no dashes. A pod some other
// controller owns that happens to match the selector is left out.
func ownedBy(ref *metav1.OwnerReference, kind, name string, obj metav1.Object) bool {
	switch kind {
	case "deployment":
		hash, ok := strings.CutPrefix(ref.Name, name+"-")
		return ref.Kind == "ReplicaSet" && ok && hash != "" && !strings.Contains(hash, "-")
	case "statefulset":
		return ref.Kind == "StatefulSet" && ref.UID == obj.GetUID()
	case "daemonset":
		return ref.Kind == "DaemonSet" && ref.UID == obj.GetUID()
	}
	return false
}

func replicasOf(r *int32) int32 {
	if r == nil {
		return 1
	}
	return *r
}

func condition(typ, st, reason, message string, at metav1.Time) workloadCondition {
	c := workloadCondition{Type: typ, Status: st, Reason: reason, Message: capText(message, maxConditionMessage)}
	if !at.IsZero() {
		t := at.UTC()
		c.LastTransition = &t
	}
	return c
}

// failingWaitReasons are the waiting reasons of a container that is
// failing rather than starting.
var failingWaitReasons = []string{
	"CrashLoopBackOff", "ImagePullBackOff", "ErrImagePull", "InvalidImageName",
	"CreateContainerConfigError", "CreateContainerError", "RunContainerError",
}

func describePod(p *corev1.Pod, now time.Time) podStatus {
	ps := podStatus{Name: p.Name, Phase: string(p.Status.Phase), Node: p.Spec.NodeName, Ready: podReady(p)}
	if p.Status.StartTime != nil {
		t := p.Status.StartTime.UTC()
		ps.StartedAt = &t
	}
	for _, cs := range p.Status.ContainerStatuses {
		c := containerStatus{Name: cs.Name, Ready: cs.Ready, RestartCount: cs.RestartCount, State: "unknown"}
		ps.Restarts += cs.RestartCount
		switch {
		case cs.State.Waiting != nil:
			c.State, c.WaitingReason = "waiting", cs.State.Waiting.Reason
		case cs.State.Running != nil:
			c.State = "running"
		case cs.State.Terminated != nil:
			c.State = "terminated"
		}
		if t := cs.LastTerminationState.Terminated; t != nil {
			c.LastTerminationReason = t.Reason
			c.LastExitCode = &t.ExitCode
			if !t.FinishedAt.IsZero() {
				f := t.FinishedAt.UTC()
				c.LastFinishedAt = &f
			}
		}
		failing := slices.Contains(failingWaitReasons, c.WaitingReason) || (!cs.Ready && cs.RestartCount > 0)
		if failing {
			switch {
			case cs.RestartCount > 0 && ps.StartedAt != nil:
				c.FailingSince = ps.StartedAt
			case c.LastFinishedAt != nil:
				c.FailingSince = c.LastFinishedAt
			default:
				c.FailingSince = ps.StartedAt
			}
		}
		if c.FailingSince != nil && (ps.FailingSince == nil || c.FailingSince.Before(*ps.FailingSince)) {
			ps.FailingSince = c.FailingSince
		}
		ps.Containers = append(ps.Containers, c)
	}
	if ps.FailingSince != nil {
		ps.FailingForSeconds = int64(now.Sub(*ps.FailingSince).Seconds())
	}
	return ps
}

func podReady(p *corev1.Pod) bool {
	for _, c := range p.Status.Conditions {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

type podLogs struct {
	Namespace string   `json:"namespace"`
	Pod       string   `json:"pod"`
	Container string   `json:"container"`
	Previous  bool     `json:"previous"`
	Lines     []string `json:"lines"`
	// Redacted says at least one line had something that looked like a
	// credential replaced.
	Redacted bool `json:"redacted"`
	// Truncated says lines were cut at 2 KiB, or older lines dropped to
	// keep the whole under its cap.
	Truncated bool `json:"truncated"`
	Untrusted bool `json:"untrusted"`
}

func (e *ToolEngine) podLogs(ctx context.Context, req pluginsdk.ToolRequest) (*podLogs, error) {
	ns, name := req.StringParam("namespace"), req.StringParam("pod")
	pod, err := e.client.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("reading pod/%s/%s: %w", ns, name, err)
	}
	if !e.selector.Matches(labels.Set(pod.Labels)) {
		return nil, fmt.Errorf("pod/%s/%s does not match tools.selector %q: %w", ns, name, e.config.Selector, ErrOutOfScope)
	}
	container := req.StringParam("container")
	if container == "" {
		if len(pod.Spec.Containers) == 0 {
			return nil, fmt.Errorf("pod/%s/%s has no containers", ns, name)
		}
		container = pod.Spec.Containers[0].Name
	}
	// Only the pod's own containers, init containers included; never an
	// ephemeral debug container someone attached.
	known := slices.ContainsFunc(pod.Spec.Containers, func(c corev1.Container) bool { return c.Name == container }) ||
		slices.ContainsFunc(pod.Spec.InitContainers, func(c corev1.Container) bool { return c.Name == container })
	if !known {
		return nil, fmt.Errorf("pod/%s/%s has no container %q", ns, name, container)
	}
	tail, _ := req.IntParam("tail_lines")
	limit := int64(maxLogFetch)
	stream, err := e.client.CoreV1().Pods(ns).GetLogs(name, &corev1.PodLogOptions{
		Container: container, TailLines: &tail, Previous: req.BoolParam("previous"), LimitBytes: &limit,
	}).Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("reading the logs of pod/%s/%s: %w", ns, name, err)
	}
	defer func() { _ = stream.Close() }()

	lines, redacted, truncated, err := readLogLines(stream, int(tail))
	if err != nil {
		return nil, fmt.Errorf("reading the logs of pod/%s/%s: %w", ns, name, err)
	}
	return &podLogs{Namespace: ns, Pod: name, Container: container, Previous: req.BoolParam("previous"),
		Lines: lines, Redacted: redacted, Truncated: truncated, Untrusted: true}, nil
}

// readLogLines reads at most tail lines, redacts each before anything else
// is done with it, caps each at maxLogLine and keeps the newest lines that
// fit in maxLogBytes.
func readLogLines(r io.Reader, tail int) (lines []string, redacted, truncated bool, err error) {
	br := bufio.NewReaderSize(r, maxLogScan)
	var red redactor
	for {
		line, cut, rerr := readLine(br)
		if rerr != nil && !errors.Is(rerr, io.EOF) {
			return nil, false, false, rerr
		}
		if line != nil || !errors.Is(rerr, io.EOF) {
			clean, changed := red.line(string(line))
			redacted = redacted || changed
			clean = stripControl(clean)
			if len(clean) > maxLogLine || cut {
				clean, truncated = capText(clean, maxLogLine), true
			}
			lines = append(lines, clean)
		}
		if errors.Is(rerr, io.EOF) {
			break
		}
	}
	if len(lines) > tail {
		lines, truncated = lines[len(lines)-tail:], true
	}
	total := 0
	for i := len(lines) - 1; i >= 0; i-- {
		if total += len(lines[i]); total > maxLogBytes {
			lines, truncated = lines[i+1:], true
			break
		}
	}
	return lines, redacted, truncated, nil
}

// readLine returns one line without its newline, at most maxLogScan bytes
// of it, discarding the rest and saying so. A final line with no newline is
// returned with io.EOF; nil with io.EOF means there was nothing left.
func readLine(br *bufio.Reader) ([]byte, bool, error) {
	var line []byte
	cut := false
	for {
		frag, err := br.ReadSlice('\n')
		if len(line) < maxLogScan {
			line = append(line, frag[:min(len(frag), maxLogScan-len(line))]...)
		}
		if len(line) >= maxLogScan {
			cut = true
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if err != nil {
			if len(line) == 0 {
				return nil, cut, err
			}
			return line, cut, err
		}
		return []byte(strings.TrimRight(string(line), "\r\n")), cut, nil
	}
}

// redactor replaces what looks like a credential in log lines. It keeps
// state across lines only for PEM blocks, whose body spans several. It is a
// best effort over common shapes, not a guarantee: logs stay untrusted and
// are read by people with access to the cluster anyway, and the aim is that
// a secret a workload printed by mistake does not travel on to Marmot.
type redactor struct {
	inPEM bool
}

const redactedMark = "[REDACTED]"

var (
	pemBegin = regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY( BLOCK)?-----`)
	pemEnd   = regexp.MustCompile(`-----END [A-Z0-9 ]*PRIVATE KEY( BLOCK)?-----`)
	// Each pattern keeps its first group, the label, and replaces the rest.
	redactions = []*regexp.Regexp{
		// Authorization headers, whatever the scheme, to the end of the
		// quoted value or the line.
		regexp.MustCompile(`(?i)(authorization"?\s*[:=]\s*"?)[^"\r\n]+`),
		regexp.MustCompile(`(?i)(\bbearer\s+)[A-Za-z0-9\-._~+/]+=*`),
		// JSON and YAML style "password": "value".
		regexp.MustCompile(`(?i)("[a-z0-9_\-]*(?:password|passwd|secret|token|api_?key|access_?key)[a-z0-9_\-]*"\s*:\s*")[^"]*`),
		// key=value and key: value, unquoted or quoted.
		regexp.MustCompile(`(?i)(\b[a-z0-9_\-]*(?:password|passwd|pwd|secret|token|api_?key|access_?key)[a-z0-9_\-]*\s*[=:]\s*"?)[^\s"&,;]+`),
		regexp.MustCompile(`(?i)(aws_secret_access_key\s*[=:]\s*"?)[A-Za-z0-9/+=]+`),
	}
	awsKeyID = regexp.MustCompile(`\b(?:AKIA|ASIA)[A-Z0-9]{16}\b`)
	jwt      = regexp.MustCompile(`\beyJ[A-Za-z0-9_\-]{4,}\.[A-Za-z0-9_\-]{4,}\.[A-Za-z0-9_\-]{4,}`)
)

// line redacts one line and reports whether anything was replaced.
func (r *redactor) line(s string) (string, bool) {
	if r.inPEM {
		if pemEnd.MatchString(s) {
			r.inPEM = false
		}
		return redactedMark, true
	}
	if loc := pemBegin.FindStringIndex(s); loc != nil {
		r.inPEM = !pemEnd.MatchString(s[loc[1]:])
		return s[:loc[0]] + redactedMark, true
	}
	out := s
	for _, re := range redactions {
		out = re.ReplaceAllString(out, "${1}"+redactedMark)
	}
	out = awsKeyID.ReplaceAllString(out, redactedMark)
	out = jwt.ReplaceAllString(out, redactedMark)
	return out, out != s
}

// stripControl drops control characters other than tab, such as terminal
// escapes, which could restyle or hide text where the line is shown.
func stripControl(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\t' {
			return r
		}
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0) {
			return -1
		}
		return r
	}, strings.ToValidUTF8(s, ""))
}

// capText cuts s to at most n bytes on a character boundary.
func capText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

type eventObject struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type eventSummary struct {
	Type      string      `json:"type"`
	Reason    string      `json:"reason"`
	Count     int32       `json:"count"`
	FirstSeen *time.Time  `json:"first_seen,omitempty"`
	LastSeen  *time.Time  `json:"last_seen,omitempty"`
	Object    eventObject `json:"object"`
	Message   string      `json:"message"`
}

type recentEvents struct {
	Namespace string         `json:"namespace"`
	Object    *eventObject   `json:"object,omitempty"`
	Events    []eventSummary `json:"events"`
	Untrusted bool           `json:"untrusted"`
}

// involvedKinds maps the involved_kind parameter to the API kind.
var involvedKinds = map[string]string{
	"pod": "Pod", "deployment": "Deployment", "statefulset": "StatefulSet", "daemonset": "DaemonSet", "job": "Job", "cronjob": "CronJob",
}

func (e *ToolEngine) recentEvents(ctx context.Context, req pluginsdk.ToolRequest) (*recentEvents, error) {
	ns := req.StringParam("namespace")
	kindParam, name := req.StringParam("involved_kind"), req.StringParam("involved_name")
	if (kindParam == "") != (name == "") {
		return nil, errors.New("involved_kind and involved_name are given together or not at all")
	}
	out := &recentEvents{Namespace: ns, Events: []eventSummary{}, Untrusted: true}
	opts := metav1.ListOptions{}
	if kindParam != "" {
		kind := involvedKinds[kindParam]
		out.Object = &eventObject{Kind: kind, Name: name}
		opts.FieldSelector = fields.SelectorFromSet(fields.Set{"involvedObject.kind": kind, "involvedObject.name": name}).String()
		// The selector scopes objects, and events carry none of their
		// labels, so with a selector set only events about an object that
		// matches it are read.
		if !e.selector.Empty() {
			objLabels, err := e.involvedLabels(ctx, ns, kindParam, name)
			if err != nil {
				return nil, err
			}
			if !e.selector.Matches(labels.Set(objLabels)) {
				return nil, fmt.Errorf("%s/%s/%s does not match tools.selector %q: %w", kindParam, ns, name, e.config.Selector, ErrOutOfScope)
			}
		}
	} else if !e.selector.Empty() {
		return nil, fmt.Errorf("with tools.selector set, recent_events reads one object's events: give involved_kind and involved_name: %w", ErrOutOfScope)
	}

	list, err := e.client.CoreV1().Events(ns).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("listing events in %s: %w", ns, err)
	}
	limit, _ := req.IntParam("limit")
	events := list.Items
	sort.SliceStable(events, func(i, j int) bool { return eventLastSeen(&events[i]).After(eventLastSeen(&events[j])) })
	for i := range events {
		ev := &events[i]
		// The field selector is the API server's filter; this is the
		// plugin's, so a server or proxy ignoring it cannot widen the read.
		if out.Object != nil && (ev.InvolvedObject.Kind != out.Object.Kind || ev.InvolvedObject.Name != out.Object.Name) {
			continue
		}
		if int64(len(out.Events)) >= limit {
			break
		}
		s := eventSummary{
			Type: ev.Type, Reason: ev.Reason, Count: max(ev.Count, 1),
			Object:  eventObject{Kind: ev.InvolvedObject.Kind, Name: ev.InvolvedObject.Name},
			Message: capText(stripControl(ev.Message), maxEventMessage),
		}
		if ev.Series != nil {
			s.Count = max(s.Count, ev.Series.Count)
		}
		if !ev.FirstTimestamp.IsZero() {
			t := ev.FirstTimestamp.UTC()
			s.FirstSeen = &t
		}
		if last := eventLastSeen(ev); !last.IsZero() {
			t := last.UTC()
			s.LastSeen = &t
		}
		out.Events = append(out.Events, s)
	}
	return out, nil
}

// eventLastSeen is when an event last happened, whichever of the API's
// three timestamps the reporter filled in.
func eventLastSeen(ev *corev1.Event) time.Time {
	switch {
	case ev.Series != nil && !ev.Series.LastObservedTime.IsZero():
		return ev.Series.LastObservedTime.Time
	case !ev.LastTimestamp.IsZero():
		return ev.LastTimestamp.Time
	case !ev.EventTime.IsZero():
		return ev.EventTime.Time
	}
	return ev.CreationTimestamp.Time
}

func (e *ToolEngine) involvedLabels(ctx context.Context, ns, kind, name string) (map[string]string, error) {
	var obj metav1.Object
	var err error
	switch kind {
	case "pod":
		obj, err = e.client.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
	case "deployment":
		obj, err = e.client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	case "statefulset":
		obj, err = e.client.AppsV1().StatefulSets(ns).Get(ctx, name, metav1.GetOptions{})
	case "daemonset":
		obj, err = e.client.AppsV1().DaemonSets(ns).Get(ctx, name, metav1.GetOptions{})
	case "job":
		obj, err = e.client.BatchV1().Jobs(ns).Get(ctx, name, metav1.GetOptions{})
	case "cronjob":
		obj, err = e.client.BatchV1().CronJobs(ns).Get(ctx, name, metav1.GetOptions{})
	default:
		return nil, fmt.Errorf("unknown kind %q", kind)
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s/%s/%s: %w", kind, ns, name, err)
	}
	return obj.GetLabels(), nil
}
