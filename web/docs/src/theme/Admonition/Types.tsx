import React, { type ReactNode } from "react";
import type { Props } from "@theme/Admonition";
import type AdmonitionTypesType from "@theme/Admonition/Types";
import { Callout, type CalloutTone } from "@site/src/components/ui/Callout";

/*
 * Admonitions (:::note, :::tip, :::info, :::warning, :::danger) as the design system's quiet
 * callout: the status soft fill, an icon and a sentence-case word. Infima's alert classes and the
 * coloured stripe are left out.
 */
function make(tone: CalloutTone, word: string, icon?: CalloutTone | "tip") {
  return function AdmonitionType({ title, children, className, id }: Props): ReactNode {
    return (
      <Callout
        tone={tone}
        icon={icon}
        title={title ?? word}
        className={className}
        id={id}
      >
        {children}
      </Callout>
    );
  };
}

const AdmonitionTypes: typeof AdmonitionTypesType = {
  note: make("neutral", "Note"),
  tip: make("success", "Tip", "tip"),
  info: make("info", "Info"),
  warning: make("warning", "Warning"),
  danger: make("danger", "Danger"),
  // Docusaurus's legacy aliases.
  secondary: make("neutral", "Note"),
  important: make("info", "Important"),
  success: make("success", "Success"),
  caution: make("warning", "Caution"),
};

export default AdmonitionTypes;
