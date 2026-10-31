import React, { type ReactNode } from "react";
import CodeBlock from "@theme-original/CodeBlock";
import type CodeBlockType from "@theme/CodeBlock";
import type { WrapperProps } from "@docusaurus/types";

type Props = WrapperProps<typeof CodeBlockType>;

// Code blocks use the design system's fixed editor palette (src/theme/prismTheme.ts), the same in
// both themes, so nothing here needs to re-render when the theme changes.
export default function CodeBlockWrapper(props: Props): ReactNode {
  return <CodeBlock {...props} />;
}
