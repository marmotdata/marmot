package presto

import (
	"errors"
	"io"
	"testing"

	prestodriver "github.com/prestodb/presto-go-client/presto"
	"github.com/stretchr/testify/assert"
)

func TestBuildDSN_PlainHTTP(t *testing.T) {
	dsn := buildDSN(&Config{Host: "presto.example.com", Port: 8080, User: "marmot"})
	assert.Equal(t, "http://marmot@presto.example.com:8080?source=marmot", dsn)
}

func TestBuildDSN_HTTPSWithPassword(t *testing.T) {
	dsn := buildDSN(&Config{Host: "presto.example.com", Port: 8443, User: "marmot", Password: "s3cret", Secure: true})
	assert.Equal(t, "https://marmot:s3cret@presto.example.com:8443?source=marmot", dsn)
}

func TestBuildDSN_EscapesPasswordCharacters(t *testing.T) {
	dsn := buildDSN(&Config{Host: "h", Port: 8443, User: "marmot", Password: "p@ss/w:rd", Secure: true})
	assert.Equal(t, "https://marmot:p%40ss%2Fw%3Ard@h:8443?source=marmot", dsn)
}

func TestBuildDSN_AddsSSLCertPath(t *testing.T) {
	dsn := buildDSN(&Config{Host: "h", Port: 8443, User: "marmot", Secure: true, SSLCertPath: "/etc/ssl/presto.pem"})
	assert.Equal(t, "https://marmot@h:8443?SSLCertPath=%2Fetc%2Fssl%2Fpresto.pem&source=marmot", dsn)
}

func TestBuildDSN_BracketsIPv6Hosts(t *testing.T) {
	dsn := buildDSN(&Config{Host: "::1", Port: 8080, User: "marmot"})
	assert.Equal(t, "http://marmot@[::1]:8080?source=marmot", dsn)
}

func TestIgnoreEOF_DriverEOFIsNotAnError(t *testing.T) {
	// The driver ends a result set with its own EOF type whose message
	// is the query id; database/sql hands it back through rows.Err().
	assert.NoError(t, ignoreEOF(&prestodriver.EOF{QueryID: "20260908_022329_00031_tihzm"}))
}

func TestIgnoreEOF_PassesOtherErrorsThrough(t *testing.T) {
	assert.ErrorIs(t, ignoreEOF(io.ErrUnexpectedEOF), io.ErrUnexpectedEOF)
	assert.NoError(t, ignoreEOF(nil))
}

func TestIgnoreEOF_SeesThroughWrapping(t *testing.T) {
	wrapped := errors.Join(errors.New("outer"), &prestodriver.EOF{QueryID: "q"})
	assert.NoError(t, ignoreEOF(wrapped))
}

func TestQuoteIdentifier(t *testing.T) {
	assert.Equal(t, `"catalog"`, quoteIdentifier("catalog"))
	assert.Equal(t, `"my""catalog"`, quoteIdentifier(`my"catalog`))
	assert.Equal(t, `"ab"`, quoteIdentifier("a\x00b"))
}

func TestQualifiedName(t *testing.T) {
	assert.Equal(t, `"memory"."shop"."orders"`, qualifiedName("memory", "shop", "orders"))
}

func TestEscapeString(t *testing.T) {
	assert.Equal(t, "hello", escapeString("hello"))
	assert.Equal(t, "it''s", escapeString("it's"))
}
