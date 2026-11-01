package kafkaconnect

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoReplacement_ConvertsNumberedGroups(t *testing.T) {
	assert.Equal(t, "${1}.${3}", goReplacement("$1.$3"))
}

func TestGoReplacement_KeepsMultiDigitGroupsTogether(t *testing.T) {
	assert.Equal(t, "${12}", goReplacement("$12"))
}

func TestGoReplacement_KeepsNamedGroups(t *testing.T) {
	assert.Equal(t, "prefix-${table}", goReplacement("prefix-${table}"))
}

func TestGoReplacement_EscapedDollarIsLiteral(t *testing.T) {
	assert.Equal(t, "cost$$", goReplacement(`cost\$`))
}

func TestGoReplacement_BackslashEscapesTheNextCharacter(t *testing.T) {
	assert.Equal(t, "a.b", goReplacement(`a\.b`))
}

func TestGoReplacement_LoneDollarIsLiteral(t *testing.T) {
	assert.Equal(t, "price$$", goReplacement("price$"))
}

func TestRegexRouters_ReadsTransformsInDeclarationOrder(t *testing.T) {
	cfg := connectorConfig{
		"transforms":                    "second,first",
		"transforms.first.type":         "org.apache.kafka.connect.transforms.RegexRouter",
		"transforms.first.regex":        "(.*)",
		"transforms.first.replacement":  "$1",
		"transforms.second.type":        "org.apache.kafka.connect.transforms.RegexRouter",
		"transforms.second.regex":       "(.*)",
		"transforms.second.replacement": "$1",
	}

	routers := regexRouters(cfg)
	require.Len(t, routers, 2)
	assert.Equal(t, "second", routers[0].alias)
	assert.Equal(t, "first", routers[1].alias)
}

func TestRegexRouters_IgnoresOtherTransforms(t *testing.T) {
	cfg := connectorConfig{
		"transforms":                   "unwrap,route",
		"transforms.unwrap.type":       "io.debezium.transforms.ExtractNewRecordState",
		"transforms.route.type":        "org.apache.kafka.connect.transforms.RegexRouter",
		"transforms.route.regex":       "(.*)",
		"transforms.route.replacement": "$1",
	}

	routers := regexRouters(cfg)
	require.Len(t, routers, 1)
	assert.Equal(t, "route", routers[0].alias)
}

func TestRegexRouters_SkipsAPatternGoCannotCompile(t *testing.T) {
	cfg := connectorConfig{
		"transforms":                   "route",
		"transforms.route.type":        "org.apache.kafka.connect.transforms.RegexRouter",
		"transforms.route.regex":       `(?=lookahead)(.*)`,
		"transforms.route.replacement": "$1",
	}

	assert.Empty(t, regexRouters(cfg))
}

func TestRoute_RewritesAFullMatch(t *testing.T) {
	cfg := connectorConfig{
		"transforms":                   "route",
		"transforms.route.type":        "org.apache.kafka.connect.transforms.RegexRouter",
		"transforms.route.regex":       `([^.]+)\.([^.]+)\.([^.]+)`,
		"transforms.route.replacement": "$1_$3",
	}

	assert.Equal(t, "shop_orders", route(regexRouters(cfg), "shop.public.orders"))
}

func TestRoute_LeavesATopicThePatternDoesNotFullyMatch(t *testing.T) {
	cfg := connectorConfig{
		"transforms":                   "route",
		"transforms.route.type":        "org.apache.kafka.connect.transforms.RegexRouter",
		"transforms.route.regex":       `shop\.(.*)`,
		"transforms.route.replacement": "$1",
	}

	assert.Equal(t, "other.public.orders", route(regexRouters(cfg), "other.public.orders"))
}

func TestRoute_AppliesRoutersInOrder(t *testing.T) {
	cfg := connectorConfig{
		"transforms":                    "strip,prefix",
		"transforms.strip.type":         "org.apache.kafka.connect.transforms.RegexRouter",
		"transforms.strip.regex":        `shop\.public\.(.*)`,
		"transforms.strip.replacement":  "$1",
		"transforms.prefix.type":        "org.apache.kafka.connect.transforms.RegexRouter",
		"transforms.prefix.regex":       "(.*)",
		"transforms.prefix.replacement": "raw_$1",
	}

	assert.Equal(t, "raw_orders", route(regexRouters(cfg), "shop.public.orders"))
}

func TestRoute_SupportsJavaNamedGroups(t *testing.T) {
	cfg := connectorConfig{
		"transforms":                   "route",
		"transforms.route.type":        "org.apache.kafka.connect.transforms.RegexRouter",
		"transforms.route.regex":       `(?<server>[^.]+)\.(?<schema>[^.]+)\.(?<table>[^.]+)`,
		"transforms.route.replacement": "${table}",
	}

	assert.Equal(t, "orders", route(regexRouters(cfg), "shop.public.orders"))
}

func TestRoute_NoRoutersReturnsTheTopicUnchanged(t *testing.T) {
	assert.Equal(t, "orders", route(nil, "orders"))
}

func TestOutboxRoute_NotUsed(t *testing.T) {
	uses, topic := outboxRoute(connectorConfig{"transforms": "unwrap", "transforms.unwrap.type": "io.debezium.transforms.ExtractNewRecordState"})

	assert.False(t, uses)
	assert.Empty(t, topic)
}

func TestOutboxRoute_StaticReplacementIsTheTopic(t *testing.T) {
	uses, topic := outboxRoute(connectorConfig{
		"transforms":             "outbox",
		"transforms.outbox.type": "io.debezium.transforms.outbox.EventRouter",
		"transforms.outbox.route.topic.replacement": "domain-events",
	})

	assert.True(t, uses)
	assert.Equal(t, "domain-events", topic)
}

func TestOutboxRoute_DynamicReplacementHasNoTopic(t *testing.T) {
	uses, topic := outboxRoute(connectorConfig{
		"transforms":             "outbox",
		"transforms.outbox.type": "io.debezium.transforms.outbox.EventRouter",
		"transforms.outbox.route.topic.replacement": "outbox.event.${routedByValue}",
	})

	assert.True(t, uses)
	assert.Empty(t, topic)
}

func TestOutboxRoute_DefaultRouteIsDynamic(t *testing.T) {
	uses, topic := outboxRoute(connectorConfig{
		"transforms":             "outbox",
		"transforms.outbox.type": "io.debezium.transforms.outbox.EventRouter",
	})

	assert.True(t, uses)
	assert.Empty(t, topic)
}
