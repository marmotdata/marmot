import React, { useEffect } from "react";
import Person from "~icons/material-symbols/person-outline";
import SmartToy from "~icons/material-symbols/smart-toy-outline";
import Code from "~icons/material-symbols/code";
import Inventory from "~icons/material-symbols/inventory-2-outline";
import MenuBook from "~icons/material-symbols/menu-book-outline";
import AccountTree from "~icons/material-symbols/account-tree-outline";
import Shield from "~icons/material-symbols/shield-outline";
import { ThemedImg } from "@site/src/components/ThemedImg";

/*
 * The shape of Marmot, with as few words as possible: the systems you run as their own marks, the
 * catalog in the middle as a panel with the mascot lying on its top edge, and the people, agents and
 * code that read it.
 * Provider marks are the only full-colour artwork, as in the product; people and agents take the
 * design system's tints for them (slate and burnt sienna). Stacks top to bottom on a phone.
 */

const sources: { provider: string; name: string }[] = [
  { provider: "postgresql", name: "PostgreSQL" },
  { provider: "snowflake", name: "Snowflake" },
  { provider: "bigquery", name: "BigQuery" },
  { provider: "kafka", name: "Kafka" },
  { provider: "dbt", name: "dbt" },
  { provider: "airflow", name: "Airflow" },
  { provider: "s3", name: "S3" },
  { provider: "looker", name: "Looker" },
];

// What the catalog holds, one word each.
const holds = [
  { Icon: Inventory, name: "Catalog" },
  { Icon: MenuBook, name: "Glossary" },
  { Icon: AccountTree, name: "Lineage" },
  { Icon: Shield, name: "Access" },
];

const readers = [
  { Icon: Person, name: "People", tone: "people" },
  { Icon: SmartToy, name: "Agents", tone: "agents" },
  { Icon: Code, name: "Your code", tone: "code" },
];

// A dotted line with an arrowhead, joining one column to the next.
function Flow(): JSX.Element {
  return <div className="intro-map__flow" />;
}

export default function IntroDiagram(): JSX.Element {
  useEffect(() => {
    void import("@marmotdata/icons/define.js");
    void import("@marmotdata/elements/topography.js");
  }, []);

  return (
    <figure className="intro-map">
      <div className="intro-map__grid" aria-hidden="true">
        <ul className="intro-map__sources">
          {sources.map((s) => (
            <li key={s.provider} title={s.name}>
              <mm-provider-logo provider={s.provider} decorative></mm-provider-logo>
            </li>
          ))}
          <li className="intro-map__more">+60</li>
        </ul>

        <Flow />

        <div className="intro-map__hub">
          <ThemedImg lightSrc="/img/marmot.svg" darkSrc="/img/dark-marmot.svg" alt="" className="intro-map__mascot" />
          <div className="intro-map__panel">
            <mm-topography seed="7" density="sparse" fade="edges"></mm-topography>
            <p className="intro-map__name">Marmot</p>
            <ul className="intro-map__holds">
              {holds.map(({ Icon, name }) => (
                <li key={name}>
                  <Icon />
                  {name}
                </li>
              ))}
            </ul>
          </div>
        </div>

        <Flow />

        <ul className="intro-map__readers">
          {readers.map(({ Icon, name, tone }) => (
            <li key={name} className={`intro-map__reader intro-map__reader--${tone}`}>
              <Icon />
              {name}
            </li>
          ))}
        </ul>
      </div>
      <figcaption className="mm-visually-hidden">
        Plugins read metadata from PostgreSQL, Snowflake, BigQuery, Kafka, dbt, Airflow, S3, Looker and more than 60 other systems into
        Marmot's catalog, glossary, lineage and access rules. People read it in the web UI, agents through the MCP server, and your code through the API, CLI and SDKs.
      </figcaption>
    </figure>
  );
}
