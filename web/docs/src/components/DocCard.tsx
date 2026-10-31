import React, { useId } from "react";
import clsx from "clsx";
import Link from "@docusaurus/Link";
import { useDocsVersionCandidates } from "@docusaurus/plugin-content-docs/client";
import { Icon } from "@iconify/react";
import OpenInNew from "~icons/material-symbols/open-in-new";
import ChevronRight from "~icons/material-symbols/chevron-right";

function useDocHref(docId: string | undefined, href: string | undefined): string {
  // Always call the hook (rules of hooks); ignore the result if docId is unset.
  const candidates = useDocsVersionCandidates("default");
  if (docId) {
    for (const version of candidates) {
      const doc = version.docs.find((d) => d.id === docId);
      if (doc) return doc.path;
    }
    return "#";
  }
  return href ?? "#";
}

const isExternal = (to: string) => /^https?:\/\//.test(to);

/*
 * Links to other pages are the design system's link list (.mm-link-list from @marmotdata/css, no
 * JavaScript needed): each entry is one link over the whole row, with the page's icon, the title,
 * one quiet line and a chevron, or open-in-new for another site. No cards, no tinted icon tiles.
 */

interface DocCardProps {
  title: string;
  description: string;
  /** An Iconify name, e.g. "mdi:rocket-launch". Drawn at 16px in ink-2. */
  icon?: string;
  /** Doc ID (preferred): version-aware, e.g. "Deploy/Docker-Compose" or "Plugins/index". */
  docId?: string;
  /** Escape hatch for external URLs or deliberate cross-version links. */
  href?: string;
}

function LinkListItem({
  title,
  description,
  icon,
  to,
}: {
  title: string;
  description: string;
  icon?: string;
  to?: string;
}): JSX.Element {
  const descriptionId = useId();
  const lead = icon ? <Icon icon={icon} width={16} height={16} className="mm-link-list__icon" aria-hidden="true" /> : null;
  if (!to) {
    return (
      <li>
        <div className="mm-link-list__item">
          {lead}
          <span className="mm-link-list__text">
            <span className="mm-link-list__title">{title}</span>
            <span className="mm-link-list__description">{description}</span>
          </span>
        </div>
      </li>
    );
  }
  const external = isExternal(to);
  return (
    <li>
      {/* The description is the link's accessible description, hidden from its name so it is read once. */}
      <Link to={to} className="mm-link-list__item" aria-describedby={descriptionId}>
        {lead}
        <span className="mm-link-list__text">
          <span className="mm-link-list__title">
            {title}
            {external && <span className="mm-visually-hidden"> (opens in a new tab)</span>}
          </span>
          <span className="mm-link-list__description" id={descriptionId} aria-hidden="true">
            {description}
          </span>
        </span>
        {external ? (
          <OpenInNew aria-hidden="true" className="mm-link-list__indicator" />
        ) : (
          <ChevronRight aria-hidden="true" className="mm-link-list__indicator" />
        )}
      </Link>
    </li>
  );
}

export function DocCard({ title, description, icon, docId, href }: DocCardProps): JSX.Element {
  const to = useDocHref(docId, href);
  return <LinkListItem title={title} description={description} icon={icon} to={to} />;
}

interface DocCardGridProps {
  children: React.ReactNode;
}

export function DocCardGrid({ children }: DocCardGridProps): JSX.Element {
  return <ul className="mm-link-list mm-link-list--2">{children}</ul>;
}

interface CalloutCardProps {
  title: string;
  description: string;
  buttonText: string;
  variant?: "primary" | "secondary" | "external";
  icon?: string;
  /** Doc ID (preferred): version-aware. */
  docId?: string;
  /** Escape hatch for external URLs or deliberate cross-version links. */
  href?: string;
}

/**
 * A next step: a quiet panel with a title, one sentence and one button. Only the primary variant
 * uses the burnt-sienna button; the others are secondary, so a page never has two loud actions.
 */
export function CalloutCard({
  title,
  description,
  docId,
  href,
  buttonText,
  variant = "primary",
}: CalloutCardProps): JSX.Element {
  const to = useDocHref(docId, href);
  const external = variant === "external" || isExternal(to);
  return (
    <div className="doc-next">
      <div className="doc-next__text">
        <p className="doc-next__title">{title}</p>
        <p className="doc-next__desc">{description}</p>
      </div>
      <Link
        to={to}
        className={clsx("mm-btn", variant === "primary" ? "mm-btn--primary" : "mm-btn--secondary")}
      >
        {buttonText}
        {external && (
          <>
            <OpenInNew aria-hidden="true" />
            <span className="mm-visually-hidden">(opens in a new tab)</span>
          </>
        )}
      </Link>
    </div>
  );
}

interface FeatureCardProps {
  title: string;
  description: string;
  icon?: string;
  docId?: string;
  href?: string;
}

/** With `docId` or `href` it is a DocCard; without, plain text (title and one line) with no indicator. */
export function FeatureCard({ title, description, icon, docId, href }: FeatureCardProps): JSX.Element {
  const to = useDocHref(docId, href);
  const linked = Boolean(docId || href);
  return <LinkListItem title={title} description={description} icon={icon} to={linked ? to : undefined} />;
}

export function FeatureGrid({ children }: DocCardGridProps): JSX.Element {
  return <ul className="mm-link-list mm-link-list--2">{children}</ul>;
}
