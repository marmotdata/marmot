import React, { type ReactNode } from "react";
import clsx from "clsx";
import { useBlogPost } from "@docusaurus/plugin-content-blog/client";
import type { Props } from "@theme/BlogPostItem/Header/Info";

const dateFormat = new Intl.DateTimeFormat("en-GB", {
  day: "numeric",
  month: "long",
  year: "numeric",
  timeZone: "UTC",
});

/**
 * The caption under a post's title, in the list and on the post: "13 August 2026 · 6 min read ·
 * Bruno Schaatsbergen". Sentence case, the text face, ink-2. Authors are named here rather than in
 * a separate block with avatars.
 */
export default function BlogPostItemHeaderInfo({ className }: Props): ReactNode {
  const { metadata } = useBlogPost();
  const { date, readingTime, authors } = metadata;
  const minutes = typeof readingTime === "number" ? Math.max(1, Math.ceil(readingTime)) : undefined;
  const names = authors.map((a) => a.name).filter(Boolean);

  return (
    <p className={clsx("blog-caption", className)}>
      <time dateTime={date}>{dateFormat.format(new Date(date))}</time>
      {minutes !== undefined && <> · {minutes}&nbsp;min read</>}
      {names.length > 0 && <> · {names.join(", ")}</>}
    </p>
  );
}
