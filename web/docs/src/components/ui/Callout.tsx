import React, { type ReactNode } from "react";
import clsx from "clsx";
import InfoIcon from "~icons/material-symbols/info-outline";
import TipIcon from "~icons/material-symbols/lightbulb-outline";
import WarningIcon from "~icons/material-symbols/warning-outline";
import DangerIcon from "~icons/material-symbols/error-outline";
import SuccessIcon from "~icons/material-symbols/check-circle-outline";
import NoteIcon from "~icons/material-symbols/sticky-note-2-outline";

export type CalloutTone = "neutral" | "info" | "success" | "warning" | "danger";

const ICONS: Record<CalloutTone | "tip", (props: React.SVGProps<SVGSVGElement>) => ReactNode> = {
  neutral: NoteIcon,
  info: InfoIcon,
  tip: TipIcon,
  success: SuccessIcon,
  warning: WarningIcon,
  danger: DangerIcon,
};

/**
 * The design system's callout (mm-callout): a soft status fill, an icon and a word, no stripe and no
 * border. Admonitions (:::note, :::tip…) and TipBox both render through this, so they read the same.
 */
export function Callout({
  tone,
  icon,
  title,
  children,
  className,
  id,
}: {
  tone: CalloutTone;
  /** Which icon to draw, when it differs from the tone (a tip is green with a lightbulb). */
  icon?: CalloutTone | "tip";
  title?: ReactNode;
  children?: ReactNode;
  className?: string;
  id?: string;
}): ReactNode {
  const Icon = ICONS[icon ?? tone];
  return (
    <div className={clsx("mm-callout", `mm-callout--${tone}`, "doc-callout", className)} id={id}>
      <Icon aria-hidden="true" />
      {title ? <p className="mm-callout__title">{title}</p> : null}
      {children ? (
        <div className="mm-callout__body">{children}</div>
      ) : null}
    </div>
  );
}
