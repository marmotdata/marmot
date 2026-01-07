import type * as PrismNamespace from "prismjs";

export default function prismIncludeLanguages(
  PrismObject: typeof PrismNamespace,
): void {
  PrismObject.languages.marmot = {
    comment: {
      pattern: /#.*/,
      greedy: true,
    },
    string: {
      pattern: /"[^"]*"|'[^']*'/,
      greedy: true,
    },
    field: {
      pattern: /@(?:metadata\.[a-zA-Z0-9_.]+|kind|type|provider|name)\b/,
    },
    boolean: {
      pattern: /\b(?:AND|OR|NOT)\b/,
      alias: "keyword",
    },
    operator: {
      pattern: /\b(?:contains|range)\b|[:<>=!]+/,
    },
    number: {
      pattern: /\b\d+\b/,
    },
    punctuation: /[[\]()]/,
  };
}
