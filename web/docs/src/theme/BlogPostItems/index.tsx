import React, { type ReactNode } from "react";
import { BlogPostProvider } from "@docusaurus/plugin-content-blog/client";
import Link from "@docusaurus/Link";
import BlogPostItemHeaderInfo from "@theme/BlogPostItem/Header/Info";
import type { Props } from "@theme/BlogPostItems";

/**
 * The blog index and tag pages: posts as a list with thin rules, each with its banner as a
 * thumbnail, its title and one caption.
 */
export default function BlogPostItems({ items }: Props): ReactNode {
  return (
    <ol className="blog-list">
      {items.map(({ content: BlogPostContent }) => (
        <BlogPostProvider key={BlogPostContent.metadata.permalink} content={BlogPostContent}>
          <li className="blog-list__item">
            {BlogPostContent.frontMatter.image && (
              // The title is the link; the picture repeats it for the pointer only.
              <Link to={BlogPostContent.metadata.permalink} className="blog-list__thumb" tabIndex={-1} aria-hidden="true">
                <img src={BlogPostContent.frontMatter.image} alt="" loading="lazy" />
              </Link>
            )}
            <div>
              <h2 className="blog-list__title">
                <Link to={BlogPostContent.metadata.permalink}>{BlogPostContent.metadata.title}</Link>
              </h2>
              <BlogPostItemHeaderInfo />
            </div>
          </li>
        </BlogPostProvider>
      ))}
    </ol>
  );
}
