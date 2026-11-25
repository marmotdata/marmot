package dataquery

import (
	"fmt"
	"time"
	"unicode"
	"unicode/utf8"
)

// Receipt statuses. A served source's query always records one of them: verified when the serving plugin's signed receipt checked out, invalid when one arrived and did not, and missing when none arrived, as from a serving plugin that predates receipts or a query cut off before it finished.
const (
	ReceiptVerified = "verified"
	ReceiptInvalid  = "invalid"
	ReceiptMissing  = "missing"
)

// RunnerReceipt is a serving plugin's signed account of one query it ran: what it ran, for whom, how many rows it sent and how it ended. It is kept on the query record next to Marmot's own account, so the two can be compared, and carries the signed bytes, signature and certificate so it can be verified again later against the instance's CA.
type RunnerReceipt struct {
	Status string `json:"status"`
	// Problem says why a receipt is invalid or missing.
	Problem string `json:"problem,omitempty"`
	// Enrolment is the serving plugin's enrolment, read from the certificate that signed the receipt.
	Enrolment       string     `json:"enrolment,omitempty"`
	QueryID         string     `json:"query_id,omitempty"`
	Origin          string     `json:"origin,omitempty"`
	Source          string     `json:"source,omitempty"`
	Actor           string     `json:"actor,omitempty"`
	StatementSHA256 string     `json:"statement_sha256,omitempty"`
	Nonce           string     `json:"nonce,omitempty"`
	Rows            int64      `json:"rows"`
	MaxRows         int64      `json:"max_rows,omitempty"`
	DurationMS      int64      `json:"duration_ms"`
	Error           string     `json:"error,omitempty"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
	Signed          []byte     `json:"signed,omitempty" swaggertype:"string" format:"byte"`
	Signature       []byte     `json:"signature,omitempty" swaggertype:"string" format:"byte"`
	Certificate     []byte     `json:"certificate,omitempty" swaggertype:"string" format:"byte"`
} // @name QueryRunnerReceipt

// ReceiptStream is implemented by the result streams of served sources. Receipt returns the serving plugin's receipt once the stream has ended, verified or marked invalid, or nil when none arrived.
type ReceiptStream interface {
	Receipt() *RunnerReceipt
}

// maxOriginLength matches the origin column.
const maxOriginLength = 200

// validateOrigin accepts free-form origins, such as task_run:<id>, that fit the column and carry no control characters, since they are shown in the audit and written into a serving plugin's log.
func validateOrigin(origin string) error {
	if len(origin) > maxOriginLength {
		return fmt.Errorf("%w: origin is longer than %d bytes", ErrInvalidInput, maxOriginLength)
	}
	if !utf8.ValidString(origin) {
		return fmt.Errorf("%w: origin is not valid UTF-8", ErrInvalidInput)
	}
	for _, r := range origin {
		if unicode.IsControl(r) {
			return fmt.Errorf("%w: origin contains a control character", ErrInvalidInput)
		}
	}
	return nil
}
