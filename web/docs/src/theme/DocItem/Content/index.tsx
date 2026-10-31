import React, { type ReactNode } from "react";
import clsx from "clsx";
import { ThemeClassNames } from "@docusaurus/theme-common";
import { useDoc } from "@docusaurus/plugin-content-docs/client";
import useRouteContext from "@docusaurus/useRouteContext";
import Link from "@docusaurus/Link";
import Heading from "@theme/Heading";
import MDXContent from "@theme/MDXContent";
import type { Props } from "@theme/DocItem/Content";
import ArrowBack from "~icons/material-symbols/arrow-back";

const dateFormat = new Intl.DateTimeFormat("en-GB", {
  day: "numeric",
  month: "long",
  year: "numeric",
  timeZone: "UTC",
});

/*
 * Docusaurus's DocItem/Content, with one addition: a resources article opens with a quiet link back
 * to the hub, the title, a caption with its date and its cover. Docs pages render as
 * before: the "synthetic" title when the Markdown has no h1 of its own.
 */
export default function DocItemContent({ children }: Props): ReactNode {
  const { metadata, frontMatter, contentTitle } = useDoc();
  const { plugin } = useRouteContext();
  const showTitle = !frontMatter.hide_title && typeof contentTitle === "undefined";
  const isResource = plugin.id === "resources" && !frontMatter.hide_title;
  const date = (frontMatter as { date?: string | Date }).date;

  return (
    <div className={clsx(ThemeClassNames.docs.docMarkdown, "markdown", isResource && "resource-article")}>
      {isResource ? (
        <header className="resource-article__header">
          <Link to="/resources" className="resource-article__back">
            <ArrowBack aria-hidden="true" />
            All resources
          </Link>
          {showTitle && <Heading as="h1">{metadata.title}</Heading>}
          {date && (
            <p className="resource-article__caption">
              <time dateTime={new Date(date).toISOString().slice(0, 10)}>{dateFormat.format(new Date(date))}</time>
            </p>
          )}
          {frontMatter.image && <img className="resource-article__cover" src={frontMatter.image} alt="" width={1200} height={630} />}
        </header>
      ) : (
        showTitle && (
          <header>
            <Heading as="h1">{metadata.title}</Heading>
          </header>
        )
      )}
      <MDXContent>{children}</MDXContent>
    </div>
  );
}
