package timescale

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOpaqueCalls(t *testing.T) {
	for expr, want := range map[string][]string{
		`query_to_xml('select * from payroll'::text, true, false, ''::text)`: {"query_to_xml"},
		`table_to_xml('payroll'::regclass, true, false, ''::text)`:           {"table_to_xml"},
		`public.read_payroll(o.id)`:                                          {"public.read_payroll"},
		`"Sneaky"(o.id)`:                                                     {`"Sneaky"`},
		`pg_catalog.lower((o.name)::text)`:                                   nil,
		`lower((o.name)::text)`:                                              nil,
		`COALESCE(o.total, '0'::numeric)`:                                    nil,
		`count(*)`:                                                           nil,
		// TimescaleDB hyperfunctions compute from the rows already scanned.
		`time_bucket('1 day', o.time)`:      nil,
		`first(o.value, o.time)`:            nil,
		`'x'::character varying(10)`:        nil,
		`(o.id = ANY ('{1,2}'::integer[]))`: nil,
		`'query_to_xml(''x'')'::text`:       nil,
		`E'it\'s query_to_xml(x)'::text`:    nil,
		`'unterminated`:                     {"<unreadable expression>"},
	} {
		assert.Equal(t, want, opaqueCalls(expr), expr)
	}
}
