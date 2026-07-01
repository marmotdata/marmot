import type { Config } from "@docusaurus/types";
import type * as Preset from "@docusaurus/preset-classic";
import type * as Redocusaurus from "redocusaurus";
import tailwindPlugin from "./plugins/tailwind-config.cjs";
import unpluginIconsPlugin from "./plugins/unplugin-icons.cjs";
import { lightTheme, darkTheme } from "./src/theme/prismTheme";
import * as fs from "fs";
import * as path from "path";

// This runs in Node.js - Don't use client-side code here (browser APIs, JSX...)

// Load snapshotted doc versions (managed by CI on stable releases)
let versions: string[] = [];
try {
  versions = JSON.parse(
    fs.readFileSync(path.resolve(__dirname, "versions.json"), "utf-8"),
  );
} catch {
  // No versions.json yet — only "Preview" docs exist
}

const config: Config = {
  title: "Marmot",
  tagline: "The open source context layer for agents and humans",
  favicon: "img/favicon.ico",

  // Set the production url of your site here
  url: "https://marmotdata.io",
  // Set the /<baseUrl>/ pathname under which your site is served
  // For GitHub pages deployment, it is often '/<projectName>/'
  baseUrl: "/",

  // GitHub pages deployment config.
  organizationName: "marmotdata",
  projectName: "marmot",

  onBrokenLinks: "warn",
  onBrokenMarkdownLinks: "warn",

  // Even if you don't use internationalization, you can use this field to set
  // useful metadata like html lang. For example, if your site is Chinese, you
  // may want to replace "en" with "zh-Hans".
  i18n: {
    defaultLocale: "en",
    locales: ["en"],
  },
  stylesheets: [
    {
      href: "https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700;800;900&display=swap",
      type: "text/css",
    },
  ],
  headTags: [
    {
      tagName: "meta",
      attributes: {
        name: "theme-color",
        content: "#fefcfb",
        media: "(prefers-color-scheme: light)",
      },
    },
    {
      tagName: "meta",
      attributes: {
        name: "theme-color",
        content: "#1a1a1a",
        media: "(prefers-color-scheme: dark)",
      },
    },
    ...(process.env.NODE_ENV === "production"
      ? [
          {
            tagName: "meta" as const,
            attributes: {
              "http-equiv": "Content-Security-Policy",
              content:
                "default-src 'self'; script-src 'self' 'unsafe-inline' https://challenges.cloudflare.com; worker-src 'self' blob:; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data:; connect-src 'self' https://api.iconify.design https://api.marmotdata.io https://challenges.cloudflare.com https://*.algolia.net https://*.algolianet.com https://*.algolia.com; frame-src 'self' https://giscus.app https://www.youtube.com https://challenges.cloudflare.com",
            },
          },
        ]
      : []),
    {
      tagName: "meta",
      attributes: {
        name: "referrer",
        content: "strict-origin-when-cross-origin",
      },
    },
    // Site-wide Schema.org structured data for search engines and AI assistants.
    {
      tagName: "script",
      attributes: { type: "application/ld+json" },
      innerHTML: JSON.stringify({
        "@context": "https://schema.org",
        "@graph": [
          {
            "@type": "Organization",
            "@id": "https://marmotdata.io/#organization",
            name: "Marmot",
            url: "https://marmotdata.io",
            logo: "https://marmotdata.io/img/social-card.png",
            sameAs: [
              "https://github.com/marmotdata/marmot",
              "https://discord.gg/TWCk7hVFN4",
            ],
          },
          {
            "@type": "WebSite",
            "@id": "https://marmotdata.io/#website",
            url: "https://marmotdata.io",
            name: "Marmot",
            description:
              "The open source context layer for agents and humans.",
            publisher: { "@id": "https://marmotdata.io/#organization" },
          },
          {
            "@type": "SoftwareApplication",
            name: "Marmot",
            description:
              "The open source context layer for your whole stack. Catalog every service, API, queue, topic, database and pipeline, then expose real, governed context to AI agents through MCP and to your team.",
            url: "https://marmotdata.io",
            applicationCategory: "DeveloperApplication",
            operatingSystem: "Linux, macOS, Windows",
            license: "https://github.com/marmotdata/marmot/blob/main/LICENSE",
            offers: { "@type": "Offer", price: "0", priceCurrency: "USD" },
            publisher: { "@id": "https://marmotdata.io/#organization" },
          },
        ],
      }),
    },
  ],
  plugins: [
    tailwindPlugin,
    unpluginIconsPlugin,
    [
      "@docusaurus/plugin-content-docs",
      {
        id: "resources",
        path: "resources",
        routeBasePath: "resources",
        sidebarPath: "./sidebarsResources.ts",
        showLastUpdateTime: true,
        editUrl: undefined,
      },
    ],
    [
      "docusaurus-plugin-llms",
      {
        title: "Marmot",
        description:
          "The open source context layer for agents and humans. Catalog every service, API, queue, topic, database and pipeline, then expose real, governed context to AI agents and your team.",
        docsDir: [
          { path: "docs", routeBasePath: "docs", label: "Documentation" },
          { path: "resources", routeBasePath: "resources", label: "Resources" },
        ],
        includeBlog: true,
        generateLLMsTxt: true,
        generateLLMsFullTxt: true,
        excludeImports: true,
        removeDuplicateHeadings: true,
      },
    ],
  ],
  presets: [
    [
      "classic",
      {
        docs: {
          sidebarPath: "./sidebars.ts",
          showLastUpdateTime: true,
          showLastUpdateAuthor: true,
          lastVersion: versions.length > 0 ? versions[0] : "current",
          versions: {
            current: {
              label: "Preview",
              banner: "unreleased",
            },
            ...(versions.length > 0
              ? { [versions[0]]: { label: versions[0] } }
              : {}),
          },
        },
        blog: {
          blogTitle:
            "Marmot Blog: Data Catalogs, AI Context and Open Source Metadata",
          blogDescription:
            "Engineering writing from the Marmot team on data catalogs, AI context layers, MCP, lineage and running metadata infrastructure on just Postgres.",
          showReadingTime: true,
          showLastUpdateTime: true,
          feedOptions: {
            type: ["rss", "atom"],
            xslt: true,
          },
          onInlineTags: "warn",
          onInlineAuthors: "warn",
          onUntruncatedBlogPosts: "warn",
        },
        theme: {
          customCss: "./src/css/custom.css",
        },
        sitemap: {
          lastmod: "date",
          changefreq: "weekly",
          priority: 0.5,
          filename: "sitemap.xml",
        },
      } satisfies Preset.Options,
    ],
    [
      "redocusaurus",
      {
        specs: [
          {
            spec: "../../docs/swagger.yaml",
            route: "/api/",
          },
        ],
        // Theme Options for modifying how redoc renders them
        theme: {
          // Change with your site colors
          primaryColor: "#d25a30",
        },
      },
    ] satisfies Redocusaurus.PresetEntry,
  ],

  themeConfig: {
    colorMode: {
      defaultMode: "light",
      disableSwitch: false,
      respectPrefersColorScheme: true,
    },
    image: "img/social-card.png",
    navbar: {
      title: "",
      logo: {
        alt: "Marmot",
        src: "img/marmot-text.svg",
        srcDark: "img/marmot-text-light.svg",
      },
      items: [
        { to: "/docs/introduction", label: "Docs", position: "left" },
        { to: "/pricing", label: "Pricing", position: "left" },
        { to: "/resources", label: "Resources", position: "left" },
        { to: "/blog", label: "Blog", position: "left" },
        {
          href: "https://discord.gg/TWCk7hVFN4",
          label: "Community",
          position: "left",
        },
        {
          href: "https://github.com/marmotdata/marmot",
          position: "right",
          className: "header-github-link",
          "aria-label": "GitHub repository",
        },
        {
          href: "https://demo.marmotdata.io",
          label: "Live Demo",
          position: "right",
          className: "demo-button",
        },
      ],
    },
    footer: {
      links: [
        {
          title: "Docs",
          items: [
            {
              label: "Introduction",
              to: "/docs/introduction",
            },
            {
              label: "Queries",
              to: "/docs/queries",
            },
            {
              label: "Plugins",
              to: "/docs/Plugins",
            },
            {
              label: "MCP",
              to: "/docs/MCP",
            },
          ],
        },
        {
          title: "Topics",
          items: [
            { label: "Data Catalog", to: "/resources/data-catalog" },
            { label: "Data Governance", to: "/resources/data-governance" },
            { label: "Data Quality", to: "/resources/data-quality" },
            {
              label: "AI Data Engineering",
              to: "/resources/ai-data-engineering",
            },
            { label: "MCP for Data", to: "/resources/mcp-for-data" },
          ],
        },
        {
          title: "Community",
          items: [
            {
              label: "Discord",
              href: "https://discord.gg/TWCk7hVFN4",
            },
            {
              label: "GitHub Discussions",
              href: "https://github.com/marmotdata/marmot/discussions",
            },
            {
              label: "Contact Us",
              href: "mailto:charlie@marmotdata.io",
            },
          ],
        },
        {
          title: "More",
          items: [
            {
              label: "Blog",
              to: "/blog",
            },
            {
              label: "Pricing",
              to: "/pricing",
            },
            {
              label: "Live Demo",
              href: "https://demo.marmotdata.io",
            },
            {
              label: "API Reference",
              to: "/api",
            },
            {
              label: "GitHub",
              href: "https://github.com/marmotdata/marmot/",
            },
            {
              label: "Privacy Policy",
              to: "/privacy",
            },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} Marmot.`,
    },
    algolia: {
      appId: "WAUET7ZVLV",
      apiKey: "d879e848e816790f13b0dd99333592ac",
      indexName: "Marmot Docs",
      contextualSearch: true,
      searchPagePath: "search",
    },
    prism: {
      theme: lightTheme,
      darkTheme: darkTheme,
      additionalLanguages: ["hcl"],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
