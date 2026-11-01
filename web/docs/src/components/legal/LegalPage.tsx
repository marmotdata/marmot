import React, { type ReactNode } from "react";
import Layout from "@theme/Layout";
import Link from "@docusaurus/Link";
import styles from "./LegalPage.module.css";

type Props = {
  /** The page heading, in sentence case. */
  title: string;
  /** The browser title, when it differs from the heading. */
  metaTitle?: string;
  description: string;
  /** Facts about the document: its version and when it last changed. */
  caption?: string;
  /** For an archived copy, the name and path of the current version. */
  archived?: { label: string; to: string };
  children: ReactNode;
};

export default function LegalPage({
  title,
  metaTitle = title,
  description,
  caption,
  archived,
  children,
}: Props): JSX.Element {
  return (
    <Layout title={metaTitle} description={description}>
      <div className="mkt-site">
        <article className="mm-container mkt-doc">
          <header className="mkt-doc__header">
            <h1 className="mkt-doc__title">{title}</h1>
            {caption && <p className="mkt-doc__caption">{caption}</p>}
            {archived && (
              <p className="mkt-doc__lede">
                This version is archived and kept for reference. Read the{" "}
                <Link to={archived.to}>current {archived.label}</Link>.
              </p>
            )}
          </header>
          <div className={`mkt-doc__body ${styles.body}`}>{children}</div>
        </article>
      </div>
    </Layout>
  );
}

export type Version = { label: string; to: string; detail: string };

/** The list of versions on an archive page, newest first. */
export function VersionList({ versions }: { versions: Version[] }): JSX.Element {
  return (
    <ul className={styles.versions}>
      {versions.map((v) => (
        <li key={v.to}>
          <Link className={styles.version} to={v.to}>
            {v.label}
          </Link>
          <span className={styles.versionDetail}>{v.detail}</span>
        </li>
      ))}
    </ul>
  );
}
