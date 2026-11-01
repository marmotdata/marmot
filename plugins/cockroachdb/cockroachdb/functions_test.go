package cockroachdb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOpaqueCalls(t *testing.T) {
	for expr, want := range map[string][]string{
		`order_count(id)`:                                     {"order_count"},
		`public.order_count(id)`:                              {"public.order_count"},
		`"Sneaky"(o.id)`:                                      {`"Sneaky"`},
		`pg_catalog.lower((name)::string)`:                    nil,
		`lower((name)::string)`:                               nil,
		`coalesce(total, '0'::decimal)`:                       nil,
		`count(*)`:                                            nil,
		`sum(total)`:                                          nil,
		`row_number() OVER (ORDER BY)`:                        nil,
		`(id = ANY ('{1,2}'::int[]))`:                         nil,
		`name NOT IN ('crdb_internal', 'information_schema')`: nil,
		`'order_count(x)'::string`:                            nil,
		`'unterminated`:                                       {"<unreadable expression>"},
	} {
		assert.Equal(t, want, opaqueCalls(expr), expr)
	}
}
