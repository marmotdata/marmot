import siteConfig from "@generated/docusaurus.config";
import type * as PrismNamespace from "prismjs";

export default function prismIncludeLanguages(
  PrismObject: typeof PrismNamespace,
): void {
  const {
    themeConfig: { prism },
  } = siteConfig;
  const { additionalLanguages } = prism as { additionalLanguages: string[] };

  // Prism components work on the Prism instance on the window, while prism-
  // react-renderer uses its own Prism instance. We temporarily mount the
  // instance onto window, import components to enhance it, then remove it to
  // avoid polluting global namespace.
  const PrismBefore = globalThis.Prism;
  globalThis.Prism = PrismObject;

  additionalLanguages.forEach((lang) => {
    if (lang === "php") {
      // eslint-disable-next-line global-require
      require("prismjs/components/prism-markup-templating.js");
    }
    // eslint-disable-next-line global-require, import/no-dynamic-require
    require(`prismjs/components/prism-${lang}`);
  });

  // Clear the global to avoid polluting global namespace.
  // https://github.com/PrismJS/prism/issues/1969
  delete (globalThis as { Prism?: typeof PrismNamespace }).Prism;
  if (typeof PrismBefore !== "undefined") {
    globalThis.Prism = PrismBefore;
  }

  // Custom Marmot query language used in docs/blog code blocks (```marmot).
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
