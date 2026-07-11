import React from "react";
import Link from "@docusaurus/Link";

// Single source of truth for each resource's banner image, keyed by page href.
// The in-page banner (ResourceBanner) and the homepage cards/feed both read from
// here, so an image is defined once per resource and the listing can never drift
// from the page it links to.
interface ResourceImageMeta {
  image: string;
  alt: string;
}

export const RESOURCE_IMAGES: Record<string, ResourceImageMeta> = {
  "/resources/data-catalog": {
    image: "/img/resources/data-catalog.png",
    alt: "Data catalog, what it is and how to choose one",
  },
  "/resources/ai-context-layer": {
    image: "/img/resources/ai-context-layer.png",
    alt: "AI context layer, the governed context layer for AI agents",
  },
  "/resources/data-governance": {
    image: "/img/resources/data-governance.png",
    alt: "Data governance, ownership, policy and access",
  },
  "/resources/data-quality": {
    image: "/img/resources/data-quality.png",
    alt: "Data quality, trust, freshness and certification",
  },
  "/resources/ai-data-engineering": {
    image: "/img/resources/ai-data-engineering.png",
    alt: "AI data engineering, giving agents real context",
  },
  "/resources/mcp-for-data": {
    image: "/img/resources/mcp-for-data.png",
    alt: "MCP for data, connecting agents to your catalog",
  },
  "/resources/data-catalogs-for-ai-agents": {
    image: "/img/resources/data-catalogs-for-ai-agents.png",
    alt: "Data catalogs for AI agents, the AI context layer compared",
  },
  "/resources/marmot-vs-datahub": {
    image: "/img/resources/marmot-vs-datahub.png",
    alt: "Marmot vs DataHub comparison",
  },
  "/resources/marmot-vs-openmetadata": {
    image: "/img/resources/marmot-vs-openmetadata.png",
    alt: "Marmot vs OpenMetadata comparison",
  },
  "/resources/marmot-vs-atlan": {
    image: "/img/resources/marmot-vs-atlan.png",
    alt: "Marmot vs Atlan comparison",
  },
};

function resolveImage(href: string, image?: string): ResourceImageMeta {
  if (image) return { image, alt: "" };
  return RESOURCE_IMAGES[href] ?? { image: "", alt: "" };
}

// Slim header banner shown at the top of each resource page. Reads its image
// from RESOURCE_IMAGES by href, so the page and the homepage stay in sync, and
// the banner height is set in this one place.
export function ResourceBanner({ href }: { href: string }): JSX.Element | null {
  const meta = RESOURCE_IMAGES[href];
  if (!meta) return null;
  return (
    <div style={{ margin: "0 0 2rem", borderRadius: "12px", overflow: "hidden" }}>
      <img
        src={meta.image}
        alt={meta.alt}
        style={{
          width: "100%",
          height: "350px",
          objectFit: "cover",
          objectPosition: "center",
          display: "block",
        }}
      />
    </div>
  );
}

interface ResourceHeroProps {
  eyebrow?: string;
  title: string;
  description: string;
  primaryHref?: string;
  primaryText?: string;
  secondaryHref?: string;
  secondaryText?: string;
}

// Brand-gradient hero for the /resources homepage. Replaces the old placeholder
// banner image with a CSS panel that matches the CalloutCard primary style.
export function ResourceHero({
  eyebrow,
  title,
  description,
  primaryHref,
  primaryText,
  secondaryHref,
  secondaryText,
}: ResourceHeroProps): JSX.Element {
  return (
    <div className="relative overflow-hidden rounded-2xl border border-[var(--ifm-color-primary)]/15 bg-gradient-to-br from-[var(--ifm-color-primary)]/[0.07] to-[#b34822]/[0.04] dark:border-[var(--ifm-color-primary)]/20 dark:from-[var(--ifm-color-primary)]/15 dark:to-[#b34822]/10 px-7 py-11 sm:px-12 sm:py-14 mb-10">
      <div className="relative z-10 max-w-2xl">
        {eyebrow && (
          <div className="text-xs font-semibold uppercase tracking-[0.18em] text-[var(--ifm-color-primary)] mb-3">
            {eyebrow}
          </div>
        )}
        <h1 className="text-3xl sm:text-4xl font-extrabold leading-tight text-gray-900 dark:text-white m-0">
          {title}
        </h1>
        <p className="mt-4 text-base sm:text-lg leading-relaxed text-gray-600 dark:text-gray-300 m-0">
          {description}
        </p>
        {(primaryHref || secondaryHref) && (
          <div className="mt-7 flex flex-wrap gap-3">
            {primaryHref && (
              <Link
                to={primaryHref}
                className="inline-flex items-center gap-2 rounded-lg bg-[var(--ifm-color-primary)] px-5 py-2.5 text-sm font-semibold text-white no-underline hover:no-underline hover:bg-[#b34822] transition-colors"
              >
                {primaryText}
                <span>→</span>
              </Link>
            )}
            {secondaryHref && (
              <Link
                to={secondaryHref}
                className="inline-flex items-center gap-2 rounded-lg border border-gray-300 dark:border-gray-600 px-5 py-2.5 text-sm font-semibold text-gray-700 dark:text-gray-200 no-underline hover:no-underline hover:border-[var(--ifm-color-primary)] hover:text-[var(--ifm-color-primary)] transition-colors"
              >
                {secondaryText}
              </Link>
            )}
          </div>
        )}
      </div>
      <div className="pointer-events-none absolute top-0 right-0 h-64 w-64 rounded-full bg-[var(--ifm-color-primary)]/[0.06] -translate-y-1/3 translate-x-1/3" />
      <div className="pointer-events-none absolute bottom-0 right-24 h-40 w-40 rounded-full bg-[var(--ifm-color-primary)]/[0.04] translate-y-1/3" />
    </div>
  );
}

interface ResourceCardProps {
  title: string;
  description: string;
  href: string;
  /** Optional override. Defaults to the image registered for `href`. */
  image?: string;
  /** Small label shown above the title, e.g. "Topic" or "Comparison". */
  kind?: string;
}

// Image-led card for the /resources homepage. The hub has no sidebar, so these
// cards are the primary navigation into the one level of child pages.
export function ResourceCard({
  title,
  description,
  href,
  image,
  kind,
}: ResourceCardProps): JSX.Element {
  const resolved = resolveImage(href, image);
  return (
    <Link
      to={href}
      className="group flex flex-col overflow-hidden rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 hover:border-[var(--ifm-color-primary)] hover:shadow-lg transition-all no-underline hover:no-underline"
    >
      <div className="aspect-[1200/630] overflow-hidden bg-gray-100 dark:bg-gray-900">
        <img
          src={resolved.image}
          alt={resolved.alt || title}
          loading="lazy"
          className="w-full h-full object-cover transition-transform duration-300 group-hover:scale-105"
        />
      </div>
      <div className="p-5">
        {kind && (
          <div className="text-xs font-semibold uppercase tracking-wide text-[var(--ifm-color-primary)] mb-1">
            {kind}
          </div>
        )}
        <h3 className="text-base font-semibold text-gray-900 dark:text-white m-0 group-hover:text-[var(--ifm-color-primary)] transition-colors">
          {title}
        </h3>
        <p className="mt-1 text-sm text-gray-600 dark:text-gray-400 m-0">
          {description}
        </p>
      </div>
    </Link>
  );
}

export function ResourceCardGrid({
  children,
}: {
  children: React.ReactNode;
}): JSX.Element {
  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-5 mt-6 mb-8">
      {children}
    </div>
  );
}

interface FeedItemProps {
  title: string;
  description: string;
  href: string;
  /** Optional override. Defaults to the image registered for `href`. */
  image?: string;
  /** Small label above the title, e.g. "Comparison" or "Guide". */
  kind?: string;
  /** Optional meta line, e.g. a date or reading time. */
  meta?: string;
}

// Wide, header-image feed entry for the /resources homepage. Image on the left,
// content on the right; stacks on mobile.
export function FeedItem({
  title,
  description,
  href,
  image,
  kind,
  meta,
}: FeedItemProps): JSX.Element {
  const resolved = resolveImage(href, image);
  return (
    <Link
      to={href}
      className="group flex flex-col sm:flex-row overflow-hidden rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 hover:border-[var(--ifm-color-primary)] hover:shadow-lg transition-all no-underline hover:no-underline"
    >
      <div className="sm:w-2/5 aspect-[16/9] sm:aspect-auto sm:min-h-[200px] overflow-hidden bg-gray-100 dark:bg-gray-900">
        <img
          src={resolved.image}
          alt={resolved.alt || title}
          loading="lazy"
          className="w-full h-full object-cover transition-transform duration-300 group-hover:scale-105"
        />
      </div>
      <div className="flex-1 p-6 flex flex-col justify-center">
        {kind && (
          <div className="text-xs font-semibold uppercase tracking-wide text-[var(--ifm-color-primary)] mb-1">
            {kind}
          </div>
        )}
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white m-0 group-hover:text-[var(--ifm-color-primary)] transition-colors">
          {title}
        </h3>
        <p className="mt-2 text-sm text-gray-600 dark:text-gray-400 m-0">
          {description}
        </p>
        {meta && (
          <div className="mt-2 text-xs text-gray-500 dark:text-gray-400">
            {meta}
          </div>
        )}
        <span className="mt-3 inline-flex items-center gap-1 text-sm font-medium text-[var(--ifm-color-primary)]">
          Read more
          <span className="transition-transform duration-200 group-hover:translate-x-0.5">
            →
          </span>
        </span>
      </div>
    </Link>
  );
}

export function ResourceFeed({
  children,
}: {
  children: React.ReactNode;
}): JSX.Element {
  return <div className="flex flex-col gap-5 mt-6 mb-8">{children}</div>;
}
