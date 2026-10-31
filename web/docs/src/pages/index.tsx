import React, { useEffect, useState } from "react";
import Layout from "@theme/Layout";
import Link from "@docusaurus/Link";
import { CONSOLE_URL, useCloudSignedIn } from "@site/src/hooks/useCloudSignedIn";
import MapHero from "@site/src/components/home/MapHero";
import ScreenDemo from "@site/src/components/home/ScreenDemo";
import Beats from "@site/src/components/home/Beats";
import type { Player } from "@site/src/marketing/demo";
import Faq from "@site/src/components/home/Faq";
import BeforeAfter from "@site/src/components/home/BeforeAfter";
import { findDemo, queryDemo, traceDemo, understandDemo, type DemoConfig } from "@site/src/marketing/demos";
import Search from "~icons/material-symbols/search";
import FilterList from "~icons/material-symbols/filter-list";
import Extension from "~icons/material-symbols/extension-outline";
import Group from "~icons/material-symbols/group-outline";
import MenuBook from "~icons/material-symbols/menu-book-outline";
import Verified from "~icons/material-symbols/verified-outline";
import AccountTree from "~icons/material-symbols/account-tree-outline";
import Sync from "~icons/material-symbols/sync";
import Dashboard from "~icons/material-symbols/dashboard-outline";
import Terminal from "~icons/material-symbols/terminal";
import Block from "~icons/material-symbols/block";
import History from "~icons/material-symbols/history";
import Check from "~icons/material-symbols/check";

type IconType = React.ComponentType<{ "aria-hidden"?: string }>;
type ChapterData = {
  id: string;
  title: string;
  lede: string;
  points: { Icon: IconType; text: string }[];
  /** Where to read more: the docs for what the chapter shows. */
  more: { to: string; label: string };
  demo: DemoConfig<{ focus?: string; beat?: number }>;
};
// Each chapter's demo drives a different screen; the list only has to hand them on.
const any = <S extends { focus?: string; beat?: number }>(d: DemoConfig<S>) => d as unknown as DemoConfig<{ focus?: string; beat?: number }>;

// Each chapter opens with the problem it solves, then shows the product solving it, and each
// calls back to the headline: real data, reached safely.
const chapters: ChapterData[] = [
  {
    id: "query",
    title: "Let agents query your data without handing out passwords",
    lede: "People and agents run SQL through Marmot. It checks every query against your rules before it reaches the database, and keeps a record of all of them.",
    points: [
      { Icon: Terminal, text: "SQL against the sources you connect" },
      { Icon: Block, text: "Refused before it reaches the database" },
      { Icon: History, text: "Who ran what, when, and what came back" },
    ],
    more: { to: "/docs/Cloud/access-control", label: "How access control works" },
    demo: any(queryDemo),
  },
  {
    id: "find",
    title: "Find the right table without asking in Slack",
    lede: "Search every source in plain words, then narrow it with conditions anyone can read.",
    points: [
      { Icon: Search, text: "Tables, topics, dashboards and models in one list" },
      { Icon: FilterList, text: "Filter by type, owner, tag or any metadata" },
      { Icon: Extension, text: "Kept current by more than 60 plugins" },
    ],
    more: { to: "/docs/Plugins", label: "See the plugins" },
    demo: any(findDemo),
  },
  {
    id: "understand",
    title: "Know what a column means before anyone trusts it",
    lede: "Owners, descriptions, glossary terms and certification travel with every asset, so agents answer from the same facts your team uses.",
    points: [
      { Icon: Group, text: "Owners and teams on every asset" },
      { Icon: MenuBook, text: "Glossary terms that explain each field" },
      { Icon: Verified, text: "Certified data marked as the one to use" },
    ],
    more: { to: "/docs/glossary", label: "How the glossary works" },
    demo: any(understandDemo),
  },
  {
    id: "trace",
    title: "See what breaks before you change it",
    lede: "Lineage follows data from source tables through every model to the dashboards that use it.",
    points: [
      { Icon: AccountTree, text: "Upstream and downstream, across tools" },
      { Icon: Sync, text: "From plugins and OpenLineage runs" },
      { Icon: Dashboard, text: "Dashboards that break, before they do" },
    ],
    more: { to: "/docs/open-lineage", label: "How lineage is collected" },
    demo: any(traceDemo),
  },
];

// Some of the sources Marmot has a plugin for, named beside the quote.
const sources = ["PostgreSQL", "BigQuery", "Kafka", "dbt", "Airflow", "S3", "ClickHouse", "Trino", "MongoDB"];

const deploy = {
  oss: ["The catalog, lineage, glossary and MCP server", "One binary, or on Kubernetes with the Helm chart", "Help from the community on Discord and GitHub"],
  cloud: ["Access control per table, product and term", "Governed SQL with an audit log of every query", "Upgrades, nightly backups and support from the team"],
};

// One chapter of the tour: the problem, then its points as captions that follow the demo beside it.
function Chapter({ c, flip = false }: { c: ChapterData; flip?: boolean }): JSX.Element {
  const [demo, setDemo] = useState<{ player: Player; bounds: number[] }>();
  return (
    <section className={`mkt-chapter mkt-chapter--split${flip ? " mkt-chapter--flip" : ""}`} id={c.id} aria-labelledby={`${c.id}-title`}>
      <div className="mkt-chapter__text">
        <div>
          <h2 className="mkt-chapter__title" id={`${c.id}-title`}>
            {c.title}
          </h2>
          <p className="mkt-chapter__lede">{c.lede}</p>
        </div>
        <Beats points={c.points} player={demo?.player} bounds={demo?.bounds} />
        <Link className="mm-link mkt-chapter__more" to={c.more.to}>
          {c.more.label}
        </Link>
      </div>
      <ScreenDemo {...c.demo} onPlayer={(player, bounds) => setDemo({ player, bounds })} />
    </section>
  );
}

// A pause in the tour: one line and the two ways in, so nobody scrolls three demos with nothing to act on.
function TourAction(): JSX.Element {
  return (
    <aside className="mkt-tour-action" aria-labelledby="tour-action">
      <div>
        <h2 className="mkt-tour-action__title" id="tour-action">
          Try it on your own data
        </h2>
        <p className="mkt-tour-action__lede">Launch an instance in about a minute and connect your first source.</p>
      </div>
      <div className="mkt-hero__actions">
        <SignUp />
        <Link className="mm-btn mm-btn--secondary" to="/docs/Deploy">
          Deploy it yourself
        </Link>
      </div>
    </aside>
  );
}

function SignUp({ label = "Start for free" }: { label?: string }): JSX.Element {
  const signedIn = useCloudSignedIn();
  return signedIn ? (
    <a className="mm-btn mm-btn--primary" href={CONSOLE_URL}>
      Open console
    </a>
  ) : (
    <a className="mm-btn mm-btn--primary" href="https://cloud.marmotdata.io/signup">
      {label}
    </a>
  );
}

export default function Home(): JSX.Element {
  useEffect(() => {
    void import("@marmotdata/elements/topography.js");
  }, []);
  return (
    <Layout
      title="Your agents need your data, not your database passwords"
      description="Marmot gives people and agents one governed way in to your data. It knows what every table means and who owns it, checks each query against your rules, and records every answer. Open source, or hosted on Marmot Cloud."
    >
      <div className="mkt-site">
        <section className="mkt-hero mkt-hero--map">
          <div className="mm-container mkt-hero__grid">
            <div>
              <h1 className="mkt-hero__title">Your agents need your data, not your database passwords.</h1>
              <p className="mkt-hero__lede">
                Marmot gives people and agents one governed way in to your data. It knows what every table means and who owns it, checks each
                query against your rules, and records every answer. Run it yourself or on Marmot Cloud.
              </p>
              <div className="mkt-hero__actions">
                <SignUp />
                <Link className="mm-btn mm-btn--secondary" to="/docs/Deploy">
                  Deploy it yourself
                </Link>
              </div>
            </div>
            <MapHero />
          </div>
        </section>

        <section className="mkt-proof" aria-label="Who uses Marmot">
          <div className="mm-container mkt-proof__inner">
            <figure className="mkt-quote">
              <blockquote>
                <p>
                  “Marmot brings metadata, ownership and lineage into one searchable catalog, making data products easier to discover and
                  understand.”
                </p>
              </blockquote>
              <figcaption>
                <img src="/img/testimonials/manuel-janez-garcia.jpg" alt="" width={40} height={40} loading="lazy" />
                <span>
                  <strong>Manuel Jáñez García</strong>
                  Senior Specialist of Data Governance Services, Telefónica
                </span>
              </figcaption>
            </figure>
            <div className="mkt-proof__sources">
              <p className="mkt-proof__sources-head">
                <span>Catalogs what you already run, with more than 60 plugins</span>
                <a className="mm-link" href="https://plugins.marmotdata.io">
                  Browse plugins
                </a>
              </p>
              <p className="mkt-proof__sources-list">{sources.join(", ")} and dozens more.</p>
            </div>
          </div>
        </section>

        <div className="mm-container mkt-tour">
          <Chapter c={chapters[0]!} />
        </div>

        <section className="mkt-block mkt-block--rule" aria-labelledby="change">
          <div className="mm-container">
            <div className="mkt-block__head">
              <h2 className="mkt-block__title" id="change">
                Why a database password isn’t enough
              </h2>
              <p className="mkt-block__lede">We asked finance-agent for last week’s revenue, once with a password and once through Marmot.</p>
            </div>
            <BeforeAfter />
            <div className="mkt-hero__actions mkt-block__actions">
              <SignUp />
              <Link className="mm-btn mm-btn--secondary" to="/docs/Deploy">
                Deploy it yourself
              </Link>
            </div>
          </div>
        </section>

        <div className="mm-container mkt-tour">
          {chapters.slice(1).map((c, i) => (
            <React.Fragment key={c.id}>
              <Chapter c={c} flip={i % 2 === 1} />
              {c.id === "understand" && <TourAction />}
            </React.Fragment>
          ))}
        </div>

        <section className="mkt-block mkt-block--rule" aria-labelledby="questions">
          <div className="mm-container mkt-block--aside">
            <div className="mkt-block__head">
              <h2 className="mkt-block__title" id="questions">
                Questions
              </h2>
              <p className="mkt-block__lede">
                Anything else, email{" "}
                <a className="mm-link" href="mailto:support@marmotdata.io">
                  support@marmotdata.io
                </a>
                .
              </p>
            </div>
            <Faq />
          </div>
        </section>

        <section className="mkt-band mkt-band--deploy" aria-labelledby="deploy">
          <mm-topography seed="11" fade="start"></mm-topography>
          <div className="mm-container">
            <h2 className="mkt-band__title" id="deploy">
              Give your first agent a safe way in to your data
            </h2>
            <p className="mkt-band__lede">Run it yourself or let us run it. It’s the same catalog and the same MCP server either way, so you can move between them.</p>
            <div className="mkt-deploy">
              <section className="mkt-deploy__option" aria-labelledby="deploy-cloud">
                <h3 id="deploy-cloud">Marmot Cloud</h3>
                <p>We run it for you, with governance built in.</p>
                <ul>
                  {deploy.cloud.map((t) => (
                    <li key={t}>
                      <Check aria-hidden="true" />
                      {t}
                    </li>
                  ))}
                </ul>
                <p className="mkt-deploy__price">Free to start with no card, and unlimited users on every plan.</p>
                <div className="mkt-deploy__actions">
                  <SignUp />
                  <Link className="mm-link" to="/pricing">
                    See pricing
                  </Link>
                </div>
              </section>
              <section className="mkt-deploy__option" aria-labelledby="deploy-oss">
                <h3 id="deploy-oss">Open source</h3>
                <p>MIT-licensed and free, on your own infrastructure.</p>
                <ul>
                  {deploy.oss.map((t) => (
                    <li key={t}>
                      <Check aria-hidden="true" />
                      {t}
                    </li>
                  ))}
                </ul>
                <div className="mkt-deploy__actions">
                  <Link className="mm-btn mm-btn--secondary" to="/docs/Deploy">
                    Deploy it yourself
                  </Link>
                  <a className="mm-link" href="https://github.com/marmotdata/marmot">
                    View on GitHub
                  </a>
                </div>
              </section>
            </div>
          </div>
        </section>

      </div>
    </Layout>
  );
}
