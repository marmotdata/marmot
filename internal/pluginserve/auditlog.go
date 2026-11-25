package pluginserve

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// maxAuditLine bounds one line of the audit log. Entries hold hashes rather than statements, so a real one is a few hundred bytes; anything far bigger was not written by this process.
const maxAuditLine = 64 << 10

// Audit events.
const (
	AuditPlan    = "plan"
	AuditExecute = "execute"
	AuditRefused = "refused"
	// AuditToolPlan records a tool planned. AuditToolIntent is written before a tool is executed, and the plugin is only called once it is on disk; AuditToolExecute records the outcome after. An intent with no outcome after it is a tool whose result the process never learnt, such as one cut off by a crash.
	AuditToolPlan    = "tool_plan"
	AuditToolIntent  = "tool_intent"
	AuditToolExecute = "tool_execute"
	// AuditToolRead records a read tool executed. A read changes nothing, so like a plan it is written after, with no intent before.
	AuditToolRead = "tool_read"
	// AuditTrigger records a trigger event signed and sent to Marmot, and AuditTriggerDropped events the process would not send: over the rate limit, refused by the checks, or with nowhere to go. Drops are summed per source and written at most once a minute, so a plugin that floods cannot flood the log.
	AuditTrigger        = "trigger"
	AuditTriggerDropped = "trigger_dropped"
)

// AuditEntry is one line of a serving process's audit log: a plan or execute it served, with the receipt fields for an execute, a tool planned, about to run or run, or a query or tool it refused.
type AuditEntry struct {
	Time  time.Time `json:"time"`
	Event string    `json:"event"`
	// Operation is plan, execute, tool_plan or tool_execute, for a refusal.
	Operation string `json:"operation,omitempty"`
	// Policy is the local policy's decision: allowed, allowed with what it changed, refused, none when no policy is set, or not applied when the query was refused before the policy was reached.
	Policy           string `json:"policy"`
	Reason           string `json:"reason,omitempty"`
	QueryID          string `json:"query_id,omitempty"`
	Origin           string `json:"origin,omitempty"`
	Source           string `json:"source,omitempty"`
	Actor            string `json:"actor,omitempty"`
	StatementSHA256  string `json:"statement_sha256,omitempty"`
	Nonce            string `json:"nonce,omitempty"`
	MaxRows          int64  `json:"max_rows,omitempty"`
	Rows             int64  `json:"rows"`
	DurationMS       int64  `json:"duration_ms"`
	Error            string `json:"error,omitempty"`
	ReceiptSignature string `json:"receipt_signature,omitempty"`
	// The tool fields are set for tools only. Parameters are recorded by hash, like statements, so the log holds no values that might be sensitive; the receipt Marmot keeps carries the same hash.
	Tool           string `json:"tool,omitempty"`
	ParamsSHA256   string `json:"params_sha256,omitempty"`
	Precondition   string `json:"precondition,omitempty"`
	ManifestDigest string `json:"manifest_digest,omitempty"`
	Target         string `json:"target,omitempty"`
	Summary        string `json:"summary,omitempty"`
	// The approval fields are set on an execute that was approved: the digest the approval was verified over, when it expires and who approved. A restart reads unexpired digests back from the log, so a spent approval stays spent.
	ApprovalDigest    string     `json:"approval_digest,omitempty"`
	ApprovalExpiresAt *time.Time `json:"approval_expires_at,omitempty"`
	Approvers         []string   `json:"approvers,omitempty"`
	// The trigger fields are set on trigger events and drops. Count is how many events a drop line sums.
	Trigger     string `json:"trigger,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	Sequence    uint64 `json:"sequence,omitempty"`
	Count       int64  `json:"count,omitempty"`
	// Prev is the hex SHA-256 of the previous line as written, or empty on the first line, which chains every line to all before it.
	Prev string `json:"prev"`
}

// AuditLog appends a hash-chained JSON line per served query to a file. Changing or removing any line breaks the chain at the line after it, which VerifyAuditLog reports. Removing lines from the end leaves no break in the chain, so that is caught by comparing against the receipts Marmot keeps for every execute.
type AuditLog struct {
	mu   sync.Mutex
	f    *os.File
	prev string
}

// OpenAuditLog opens or creates the log at path, readable by its owner only, and continues the chain from its last line.
func OpenAuditLog(path string) (*AuditLog, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600) //nolint:gosec // G304: the path is the operator's --audit-log flag
	if err != nil {
		return nil, fmt.Errorf("opening audit log: %w", err)
	}
	// An existing file keeps the mode it was made with, so tighten it here too.
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("restricting audit log: %w", err)
	}
	last, err := lastLine(f)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("reading audit log %s: %w", path, err)
	}
	l := &AuditLog{f: f}
	if last != nil {
		var e AuditEntry
		if err := json.Unmarshal(last, &e); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("audit log %s: its last line is not an audit entry; check it with marmot ingest verify-audit-log", path)
		}
		l.prev = lineHash(last)
	}
	return l, nil
}

// Append writes e as the next line, chained to the one before.
func (l *AuditLog) Append(e AuditEntry) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	e.Prev = l.prev
	line, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("encoding audit entry: %w", err)
	}
	// One write per line, on a file opened for appending, so a line is never interleaved with another.
	if _, err := l.f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("writing audit log: %w", err)
	}
	l.prev = lineHash(line)
	return nil
}

// Since returns the entries written at or after since, reading back from the end of the file and stopping at the first older line, so a long log costs only its recent tail. Lines are written in time order by one process; a line that does not parse is an error, since the log is no longer what this process wrote.
func (l *AuditLog) Since(since time.Time) ([]AuditEntry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	info, err := l.f.Stat()
	if err != nil {
		return nil, err
	}
	var out []AuditEntry
	var carry []byte
	for off := info.Size(); off > 0; {
		n := min(int64(64<<10), off)
		off -= n
		chunk := make([]byte, n)
		if _, err := l.f.ReadAt(chunk, off); err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		chunk = append(chunk, carry...)
		lines := bytes.Split(bytes.TrimRight(chunk, "\n"), []byte("\n"))
		// The first piece may be the end of a line that started in an earlier chunk.
		start := 0
		if off > 0 {
			carry, start = lines[0], 1
			if len(carry) > maxAuditLine {
				return nil, errors.New("audit log line too long")
			}
		} else {
			carry = nil
		}
		for i := len(lines) - 1; i >= start; i-- {
			if len(lines[i]) == 0 {
				continue
			}
			var e AuditEntry
			if err := json.Unmarshal(lines[i], &e); err != nil {
				return nil, fmt.Errorf("%w: a line does not parse", ErrAuditChainBroken)
			}
			if e.Time.Before(since) {
				return out, nil
			}
			out = append(out, e)
		}
	}
	return out, nil
}

// Close closes the file.
func (l *AuditLog) Close() error {
	return l.f.Close()
}

func lineHash(line []byte) string {
	sum := sha256.Sum256(line)
	return hex.EncodeToString(sum[:])
}

// lastLine returns the file's last line without its newline, or nil for an empty file, reading backwards from the end so a long log is not read whole at every start. A file that does not end in a newline was cut off mid-write or edited, and is refused rather than extended.
func lastLine(f *os.File) ([]byte, error) {
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := info.Size()
	if size == 0 {
		return nil, nil
	}
	var tail []byte
	for off := size; off > 0; {
		n := min(int64(4096), off)
		off -= n
		chunk := make([]byte, n)
		if _, err := f.ReadAt(chunk, off); err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		tail = append(chunk, tail...)
		if tail[len(tail)-1] != '\n' {
			return nil, errors.New("the file ends in a partial line")
		}
		if i := bytes.LastIndexByte(tail[:len(tail)-1], '\n'); i >= 0 {
			return tail[i+1 : len(tail)-1], nil
		}
		if len(tail) > maxAuditLine {
			return nil, errors.New("its last line is too long to be an audit entry")
		}
	}
	return tail[:len(tail)-1], nil
}

// ErrAuditChainBroken is returned by VerifyAuditLog when a line does not follow from the one before it.
var ErrAuditChainBroken = errors.New("audit log chain broken")

// VerifyAuditLog checks that every line of the log at path follows from the one before it, and returns how many lines it checked. The error names the first line that does not, which is the first line after a change or a removal.
func VerifyAuditLog(path string) (int, error) {
	f, err := os.Open(path) //nolint:gosec // G304: the path is the operator's own argument
	if err != nil {
		return 0, fmt.Errorf("opening audit log: %w", err)
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 4096), maxAuditLine)
	prev, n := "", 0
	for scanner.Scan() {
		n++
		line := scanner.Bytes()
		var e AuditEntry
		if err := json.Unmarshal(line, &e); err != nil {
			return n - 1, fmt.Errorf("%w: line %d is not an audit entry", ErrAuditChainBroken, n)
		}
		if e.Prev != prev {
			return n - 1, fmt.Errorf("%w at line %d: it does not follow from the line before it, which was changed or removed", ErrAuditChainBroken, n)
		}
		prev = lineHash(line)
	}
	if err := scanner.Err(); err != nil {
		return n, fmt.Errorf("%w after line %d: %v", ErrAuditChainBroken, n, err)
	}
	return n, nil
}
