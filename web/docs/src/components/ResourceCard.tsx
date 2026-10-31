import React from "react";
import Link from "@docusaurus/Link";

/*
 * The /resources hub: a page header, then plain lists with thin rules. Each entry is the title as an
 * ink link and one quiet line under it. There are no cover images; `kind`, `meta` and `image` are
 * accepted for existing content and not drawn.
 */

interface ResourceHeroProps {
  /** Kept for existing content; there are no eyebrows. */
  eyebrow?: string;
  title: string;
  description: string;
  primaryHref?: string;
  primaryText?: string;
  secondaryHref?: string;
  secondaryText?: string;
}

export function ResourceHero({
  title,
  description,
  primaryHref,
  primaryText,
  secondaryHref,
  secondaryText,
}: ResourceHeroProps): JSX.Element {
  return (
    <header className="resource-hero">
      <h1 className="resource-hero__title">{title}</h1>
      <p className="resource-hero__lede">{description}</p>
      {(primaryHref || secondaryHref) && (
        <div className="resource-hero__actions">
          {primaryHref && (
            <Link to={primaryHref} className="mm-btn mm-btn--primary">
              {primaryText}
            </Link>
          )}
          {secondaryHref && (
            <Link to={secondaryHref} className="mm-btn mm-btn--secondary">
              {secondaryText}
            </Link>
          )}
        </div>
      )}
    </header>
  );
}

interface ResourceItemProps {
  title: string;
  description: string;
  href: string;
  image?: string;
  kind?: string;
  meta?: string;
}

// Each article's cover is named after its slug, so a link to /resources/x finds /img/resources/x.png.
const coverFor = (href: string): string | undefined => {
  const slug = href.match(/^\/resources\/([\w-]+)\/?$/)?.[1];
  return slug ? `/img/resources/${slug}.png` : undefined;
};

export function ResourceItem({ title, description, href, image }: ResourceItemProps): JSX.Element {
  const cover = image ?? coverFor(href);
  return (
    <li className={`resource-list__item${cover ? " resource-list__item--cover" : ""}`}>
      {cover && (
        // The title is the link people and screen readers use; the picture repeats it for the pointer only.
        <Link to={href} className="resource-list__thumb" tabIndex={-1} aria-hidden="true">
          <img src={cover} alt="" loading="lazy" width={1200} height={630} />
        </Link>
      )}
      <span className="resource-list__text">
        <Link to={href} className="resource-list__title">
          {title}
        </Link>
        <span className="resource-list__desc">{description}</span>
      </span>
    </li>
  );
}

export function ResourceList({ children }: { children: React.ReactNode }): JSX.Element {
  return <ul className="resource-list">{children}</ul>;
}

// The names the hub's content has used; they all render the same list.
export const ResourceCard = ResourceItem;
export const FeedItem = ResourceItem;
export const ResourceCardGrid = ResourceList;
export const ResourceFeed = ResourceList;
