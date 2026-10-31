import React, { type ReactNode } from "react";
import BlogPostItemHeaderTitle from "@theme/BlogPostItem/Header/Title";
import BlogPostItemHeaderInfo from "@theme/BlogPostItem/Header/Info";

/** A post opens with its title and one quiet caption: date, reading time and who wrote it. */
export default function BlogPostItemHeader(): ReactNode {
  return (
    <header className="blog-post__header">
      <BlogPostItemHeaderTitle className="blog-post__title" />
      <BlogPostItemHeaderInfo />
    </header>
  );
}
