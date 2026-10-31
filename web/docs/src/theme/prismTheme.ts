import type { PrismTheme } from "prism-react-renderer";

/*
 * The design system's editor palette: warm near-black in both themes, with the mm-syn-* colours.
 * The values are custom properties, so code blocks follow the tokens rather than a copy of them,
 * and the light and dark themes are the same object.
 */
const editor: PrismTheme = {
  plain: {
    color: "var(--mm-code-text)",
    backgroundColor: "var(--mm-code-bg)",
  },
  styles: [
    {
      types: ["comment", "prolog", "doctype", "cdata"],
      style: { color: "var(--mm-syn-comment)", fontStyle: "italic" },
    },
    { types: ["namespace"], style: { opacity: 0.7 } },
    { types: ["punctuation", "operator"], style: { color: "var(--mm-code-text)" } },
    {
      types: ["string", "attr-value", "char", "inserted", "regex", "url"],
      style: { color: "var(--mm-syn-string)" },
    },
    {
      types: ["number", "boolean", "constant", "symbol", "entity"],
      style: { color: "var(--mm-syn-number)" },
    },
    {
      types: ["keyword", "atrule", "selector", "tag", "important", "deleted"],
      style: { color: "var(--mm-syn-keyword)" },
    },
    {
      types: ["function", "function-variable", "class-name", "builtin", "attr-name", "property", "field"],
      style: { color: "var(--mm-syn-function)" },
    },
    { types: ["variable", "parameter"], style: { color: "var(--mm-code-text)" } },
  ],
};

export const lightTheme = editor;
export const darkTheme = editor;
