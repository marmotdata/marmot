import React from "react";
import Layout from "@theme-original/DocItem/Layout";
import type LayoutType from "@theme/DocItem/Layout";
import type { WrapperProps } from "@docusaurus/types";
import Head from "@docusaurus/Head";
import { useDoc } from "@docusaurus/plugin-content-docs/client";
import useDocusaurusContext from "@docusaurus/useDocusaurusContext";

type Props = WrapperProps<typeof LayoutType>;

export default function LayoutWrapper(props: Props): JSX.Element {
  const { metadata, frontMatter } = useDoc();
  const { siteConfig } = useDocusaurusContext();

  const url = siteConfig.url + metadata.permalink;
  // Docusaurus may report lastUpdatedAt in seconds or milliseconds depending on
  // the source; normalise to milliseconds before constructing the date.
  const modified =
    typeof metadata.lastUpdatedAt === "number"
      ? new Date(
          metadata.lastUpdatedAt < 1e12
            ? metadata.lastUpdatedAt * 1000
            : metadata.lastUpdatedAt,
        ).toISOString()
      : undefined;
  const published =
    (frontMatter.date as string | undefined) ?? modified;

  const jsonLd = {
    "@context": "https://schema.org",
    "@type": "TechArticle",
    headline: metadata.title,
    description: metadata.description,
    url,
    mainEntityOfPage: { "@type": "WebPage", "@id": url },
    ...(published ? { datePublished: published } : {}),
    ...(modified ? { dateModified: modified } : {}),
    inLanguage: "en",
    author: { "@type": "Organization", name: "Marmot" },
    publisher: {
      "@type": "Organization",
      name: "Marmot",
      url: siteConfig.url,
      logo: {
        "@type": "ImageObject",
        url: `${siteConfig.url}/img/social-card.png`,
      },
    },
  };

  return (
    <>
      <Head>
        <script type="application/ld+json">{JSON.stringify(jsonLd)}</script>
      </Head>
      <Layout {...props} />
    </>
  );
}
