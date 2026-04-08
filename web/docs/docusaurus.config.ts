import type { Config } from "@docusaurus/types";
import type * as Preset from "@docusaurus/preset-classic";
import type * as Redocusaurus from "redocusaurus";
import tailwindPlugin from "./plugins/tailwind-config.cjs";
import unpluginIconsPlugin from "./plugins/unplugin-icons.cjs";
import { lightTheme, darkTheme } from "./src/theme/prismTheme";

// This runs in Node.js - Don't use client-side code here (browser APIs, JSX...)

const config: Config = {
  title: "Marmot",
  tagline: "Modern Data Discovery for Modern Teams",
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
                "default-src 'self'; script-src 'self' 'unsafe-inline' https://challenges.cloudflare.com; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data:; connect-src 'self' https://api.iconify.design https://api.marmotdata.io https://challenges.cloudflare.com; frame-src 'self' https://giscus.app https://www.youtube.com https://challenges.cloudflare.com",
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
  ],
  plugins: [tailwindPlugin, unpluginIconsPlugin],
  presets: [
    [
      "classic",
      {
        docs: {
          sidebarPath: "./sidebars.ts",
        },
        blog: {
          showReadingTime: true,
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
    //TODO: replace with social card
    image: "img/marmot.svg",
    navbar: {
      title: "",
      logo: {
        alt: "Marmot",
        src: "img/marmot-text.svg",
      },
      items: [
        { to: "/docs/introduction", label: "Docs", position: "left" },
        { to: "/pricing", label: "Pricing", position: "left" },
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
              to: "/docs/plugins",
            },
            {
              label: "MCP",
              to: "/docs/MCP",
            },
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
    prism: {
      theme: lightTheme,
      darkTheme: darkTheme,
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
