import React, { memo } from "react";
import clsx from "clsx";
import { translate } from "@docusaurus/Translate";
import {
  useVisibleBlogSidebarItems,
  BlogSidebarItemList,
} from "@docusaurus/plugin-content-blog/client";
import BlogSidebarContent from "@theme/BlogSidebar/Content";
import type { Props as BlogSidebarContentProps } from "@theme/BlogSidebar/Content";
import type { Props } from "@theme/BlogSidebar/Desktop";

const ListComponent: BlogSidebarContentProps["ListComponent"] = ({ items }) => (
  <BlogSidebarItemList
    items={items}
    ulClassName="blog-sidebar__list clean-list"
    liClassName="blog-sidebar__item"
    linkClassName="blog-sidebar__link"
    linkActiveClassName="blog-sidebar__link--active"
  />
);

/** "Recent posts": a quiet, dense list beside the posts, in sentence case. */
function BlogSidebarDesktop({ sidebar }: Props) {
  const items = useVisibleBlogSidebarItems(sidebar.items);
  return (
    <aside className="col col--3">
      <nav
        className={clsx("blog-sidebar", "thin-scrollbar")}
        aria-label={translate({
          id: "theme.blog.sidebar.navAriaLabel",
          message: "Blog recent posts navigation",
          description: "The ARIA label for recent posts in the blog sidebar",
        })}
      >
        <p className="blog-sidebar__title">{sidebar.title}</p>
        <BlogSidebarContent
          items={items}
          ListComponent={ListComponent}
          yearGroupHeadingClassName="blog-sidebar__year"
        />
      </nav>
    </aside>
  );
}

export default memo(BlogSidebarDesktop);
