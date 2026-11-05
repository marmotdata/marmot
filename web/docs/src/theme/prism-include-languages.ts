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

  // Grammars are required by name because webpack's context module for a template string did not include every component.
  const grammars: Record<string, () => unknown> = {
    bash: () => require("prismjs/components/prism-bash.js"),
    hcl: () => require("prismjs/components/prism-hcl.js"),
  };
  additionalLanguages.forEach((lang) => {
    const load = grammars[lang];
    if (!load) throw new Error(`Add a loader for the "${lang}" Prism grammar in src/theme/prism-include-languages.ts`);
    load();
    // The grammar can land on another Prism already on the window instead of the one assigned above, so copy it across.
    const loaded = (globalThis as { Prism?: typeof PrismNamespace }).Prism?.languages[lang];
    if (!PrismObject.languages[lang] && loaded) PrismObject.languages[lang] = loaded;
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
