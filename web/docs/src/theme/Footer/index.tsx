import React from "react";
import Link from "@docusaurus/Link";

// The header's destinations (Docs, Plugins, Pricing, Resources, Blog, Community) are not repeated.
const columns: { title: string; links: { label: string; href: string }[] }[] = [
  {
    title: "Marmot Cloud",
    links: [
      { label: "Create a free account", href: "https://cloud.marmotdata.io/signup" },
      { label: "Sign in", href: "https://cloud.marmotdata.io/login" },
      { label: "Status", href: "https://status.marmotdata.io" },
    ],
  },
  {
    title: "Docs",
    links: [
      { label: "Introduction", href: "/docs/introduction" },
      { label: "Deploy it yourself", href: "/docs/Deploy" },
      { label: "Queries", href: "/docs/queries" },
      { label: "MCP", href: "/docs/MCP" },
      { label: "API reference", href: "/api" },
    ],
  },
  {
    title: "Topics",
    links: [
      { label: "Data catalog", href: "/resources/data-catalog" },
      { label: "Data governance", href: "/resources/data-governance" },
      { label: "Data quality", href: "/resources/data-quality" },
      { label: "AI data engineering", href: "/resources/ai-data-engineering" },
      { label: "MCP for data", href: "/resources/mcp-for-data" },
    ],
  },
  {
    title: "Company",
    links: [
      { label: "GitHub", href: "https://github.com/marmotdata/marmot" },
      { label: "GitHub discussions", href: "https://github.com/marmotdata/marmot/discussions" },
      { label: "Contact us", href: "mailto:support@marmotdata.io" },
      { label: "Security and disclosure", href: "/security" },
      { label: "Privacy policy", href: "/privacy" },
      { label: "Terms of service", href: "/terms" },
      { label: "Data processing agreement", href: "/dpa" },
    ],
  },
];

export default function Footer(): JSX.Element {
  return (
    <footer className="mkt-footer">
      <div className="mm-container">
        <nav className="mkt-footer__nav" aria-label="Footer">
          {columns.map((c) => (
            <div key={c.title}>
              <h2>{c.title}</h2>
              <ul>
                {c.links.map((l) => (
                  <li key={l.href}>
                    {l.href.startsWith("/") ? <Link to={l.href}>{l.label}</Link> : <a href={l.href}>{l.label}</a>}
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </nav>
        <div className="mkt-footer__base">
          <span>© {new Date().getFullYear()} Marmot</span>
          <address>66 Paul St, London EC2A 4NA</address>
        </div>
      </div>
    </footer>
  );
}
