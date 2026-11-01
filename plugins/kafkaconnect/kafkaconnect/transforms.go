package kafkaconnect

import (
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

// regexRouter is one RegexRouter single message transform: a record whose
// topic matches the pattern in full is renamed by the replacement.
type regexRouter struct {
	alias       string
	pattern     *regexp.Regexp
	replacement string
}

// regexRouters returns the RegexRouter transforms a connector declares,
// in the order Connect applies them. A pattern Go cannot compile is
// skipped with a warning rather than failing the connector.
func regexRouters(cfg connectorConfig) []regexRouter {
	var routers []regexRouter
	for _, alias := range cfg.list("transforms") {
		if !strings.Contains(cfg.get("transforms."+alias+".type"), "RegexRouter") {
			continue
		}
		expr := cfg.get("transforms." + alias + ".regex")
		if expr == "" {
			continue
		}
		// RegexRouter only rewrites a topic the pattern matches in full.
		pattern, err := regexp.Compile("^(?:" + expr + ")$")
		if err != nil {
			log.Warn().Err(err).
				Str("connector", cfg.get("name")).
				Str("transform", alias).
				Msg("Skipping RegexRouter transform with a pattern Go cannot compile")
			continue
		}
		routers = append(routers, regexRouter{
			alias:       alias,
			pattern:     pattern,
			replacement: goReplacement(cfg.get("transforms." + alias + ".replacement")),
		})
	}
	return routers
}

// route runs a topic name through every router in order.
func route(routers []regexRouter, topic string) string {
	for _, r := range routers {
		if r.pattern.MatchString(topic) {
			topic = r.pattern.ReplaceAllString(topic, r.replacement)
		}
	}
	return topic
}

// goReplacement rewrites a Java replacement string into Go's syntax.
// Java reads $1 and ${name} as group references and backslash as an
// escape; Go wants ${1} (a bare $1 swallows any digit or letter that
// follows it) and treats backslash literally.
func goReplacement(java string) string {
	var b strings.Builder
	for i := 0; i < len(java); i++ {
		ch := java[i]
		switch {
		case ch == '\\' && i+1 < len(java):
			i++
			if java[i] == '$' {
				b.WriteString("$$")
			} else {
				b.WriteByte(java[i])
			}
		case ch == '$' && i+1 < len(java) && java[i+1] == '{':
			end := strings.IndexByte(java[i:], '}')
			if end < 0 {
				b.WriteString("$$")
				continue
			}
			b.WriteString(java[i : i+end+1])
			i += end
		case ch == '$' && i+1 < len(java) && isDigit(java[i+1]):
			j := i + 1
			for j < len(java) && isDigit(java[j]) {
				j++
			}
			b.WriteString("${" + java[i+1:j] + "}")
			i = j - 1
		case ch == '$':
			b.WriteString("$$")
		default:
			b.WriteByte(ch)
		}
	}
	return b.String()
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

// outboxRoute reports whether the connector uses Debezium's outbox
// EventRouter and, when the router sends every event to one fixed topic,
// that topic. The default route names the topic after a field in each
// record, so the topics cannot be known from the config alone.
func outboxRoute(cfg connectorConfig) (usesRouter bool, staticTopic string) {
	for _, alias := range cfg.list("transforms") {
		if !strings.Contains(cfg.get("transforms."+alias+".type"), "EventRouter") {
			continue
		}
		usesRouter = true
		replacement := cfg.get("transforms." + alias + ".route.topic.replacement")
		if replacement != "" && !strings.Contains(replacement, "${") {
			staticTopic = replacement
		}
	}
	return usesRouter, staticTopic
}
