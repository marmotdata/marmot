import React, { useEffect } from "react";
import Layout from "@theme/Layout";
import Link from "@docusaurus/Link";
import OpenInNew from "~icons/material-symbols/open-in-new";
import styles from "./security.module.css";

const REPORT_EMAIL = "security@marmotdata.io";
const GITHUB_ADVISORY = "https://github.com/marmotdata/marmot/security/advisories/new";

// The policy's sections, in page order, for the contents list.
const sections = [
  { id: "report", title: "Reporting a vulnerability" },
  { id: "scope", title: "Scope" },
  { id: "qualifies", title: "What qualifies" },
  { id: "not-qualifying", title: "What does not qualify" },
  { id: "rules", title: "Rules of engagement" },
  { id: "research", title: "The vulnerability research programme" },
  { id: "safe-harbor", title: "Safe harbour" },
  { id: "thanks", title: "Thank you" },
];

export default function Security(): JSX.Element {
  useEffect(() => {
    void import("@marmotdata/elements/topography.js");
  }, []);
  return (
    <Layout
      title="Security"
      description="Marmot's vulnerability disclosure and research programmes: scope, rules of engagement and how to report a vulnerability."
    >
      <div className="mkt-site">
        <header className={styles.head}>
          <mm-topography seed="23" density="sparse" fade="start"></mm-topography>
          <div className="mm-container">
            <div className={styles.headGrid}>
              <div className={styles.headText}>
                <h1 className={`mkt-doc__title ${styles.title}`}>If you find a hole in Marmot, tell us first</h1>
                <p className="mkt-doc__lede">
                  Marmot holds the map of every table, topic, service and pipeline a company has, and who owns each
                  one, so it is worth protecting well. This page is the whole of our disclosure and research
                  programmes: how to report, what is in scope and the rules.
                </p>
                <div className={styles.actions}>
                  <a className="mm-btn mm-btn--primary" href={`mailto:${REPORT_EMAIL}`}>
                    Email {REPORT_EMAIL}
                  </a>
                  <a className="mm-btn mm-btn--secondary" href={GITHUB_ADVISORY} target="_blank" rel="noopener noreferrer">
                    Report privately on GitHub
                    <OpenInNew aria-hidden="true" />
                    <span className="mm-visually-hidden">(opens in a new tab)</span>
                  </a>
                </div>
              </div>
              <img
                className={styles.mascot}
                src="/img/marmot-security-officer.png"
                alt="The Marmot mascot in a security officer uniform holding a magnifying glass"
                width={640}
                height={698}
              />
            </div>
          </div>
        </header>

        <div className={`mkt-doc ${styles.doc}`}>
          <div className="mm-container">
            <div className={styles.layout}>
              <article className="mkt-doc__body">
                <h2 id="report">Reporting a vulnerability</h2>
                <p>
                  Send your report to{" "}
                  <a href="mailto:security@marmotdata.io">security@marmotdata.io</a>.
                  Put the details in the body of the email as plain text, not in a
                  PDF; attachments are for screenshots and proof-of-concept files.
                  For issues in the open source code you can also use{" "}
                  <a
                    href="https://github.com/marmotdata/marmot/security/advisories/new"
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    GitHub's private vulnerability reporting
                    <span className="mm-visually-hidden"> (opens in a new tab)</span>
                  </a>
                  , which opens a draft advisory that only we can see.
                </p>
                <p>If possible, include:</p>
                <ul>
                  <li>Steps to reproduce</li>
                  <li>The affected version, commit or URL</li>
                  <li>An impact summary: what an attacker gains</li>
                </ul>
                <p>
                  We are a small team, so your report is read by an engineer, not a
                  triage vendor. We aim to acknowledge within two business days and
                  to tell you within a week whether we can reproduce it. Please do
                  not report security issues through Discord, GitHub issues or the
                  support inbox; those are public or read by more people than need to
                  see a vulnerability.
                </p>

                <h2 id="scope">Scope</h2>
                <p>In scope:</p>
                <ul>
                  <li>
                    <strong>The open source project</strong>: the marmot binary, the
                    official plugins, the MCP server, the Helm chart and the
                    container images we publish.
                  </li>
                  <li>
                    <strong>Marmot Cloud</strong>: cloud.marmotdata.io,
                    api.marmotdata.io, and instances on marmotdata.cloud that you
                    own.
                  </li>
                  <li>
                    <strong>How we ship</strong>: the integrity of releases and
                    plugin distribution. If you can make someone install something we
                    did not publish, we want to know about it more than almost
                    anything else on this page.
                  </li>
                </ul>
                <p>Out of scope:</p>
                <ul>
                  <li>
                    This website, marmotdata.io, and the docs. It is a static site
                    with no accounts and nothing sensitive behind it.
                  </li>
                  <li>
                    Cloud instances that are not yours. Never test against another
                    tenant, under any circumstances.
                  </li>
                  <li>
                    Third-party services we use. Report those to the third party.
                  </li>
                </ul>

                <h2 id="qualifies">What qualifies</h2>
                <p>
                  Anything that breaks the security model of the catalog or the
                  platform it runs on. The clearest examples:
                </p>
                <ul>
                  <li>
                    Tenant isolation breaks on Marmot Cloud: reading or writing
                    another tenant's metadata, configuration or credentials. This is
                    the top of our severity scale.
                  </li>
                  <li>Authentication or authorisation bypass</li>
                  <li>Privilege escalation, in the product or in Cloud</li>
                  <li>Remote code execution</li>
                  <li>
                    Injection of any kind that crosses a trust boundary, including
                    SQL injection
                  </li>
                  <li>
                    Server-side request forgery, including through plugin
                    configuration
                  </li>
                  <li>
                    Exposure of stored secrets: connection credentials, API keys,
                    session tokens
                  </li>
                  <li>
                    Supply chain issues in how we build and distribute releases and
                    plugins
                  </li>
                </ul>

                <h2 id="not-qualifying">What does not qualify</h2>
                <ul>
                  <li>
                    Automated scanner output without a working proof of concept
                  </li>
                  <li>Self-XSS, or XSS that only affects the person triggering it</li>
                  <li>Clickjacking without a demonstrated harmful action</li>
                  <li>
                    Missing security headers or cookie flags on pages with nothing
                    sensitive on them
                  </li>
                  <li>Username or email enumeration, and password policy opinions</li>
                  <li>
                    Denial of service and rate-limit testing. Do not do this at all;
                    it is against the rules below, not just non-qualifying.
                  </li>
                  <li>
                    Vulnerable dependency reports without a reachable path through
                    our code
                  </li>
                  <li>
                    Anything that requires an already-compromised machine or stolen
                    credentials
                  </li>
                  <li>
                    A self-hosted install with the safe defaults turned off. If you
                    disable authentication on your own instance, that is a decision,
                    not a finding.
                  </li>
                </ul>

                <h2 id="rules">Rules of engagement</h2>
                <ul>
                  <li>
                    Test only what you own: your own accounts, your own instances,
                    your own self-hosted deployments.
                  </li>
                  <li>
                    If you find yourself looking at another tenant's data, stop.
                    Record the minimum needed to prove the issue, report it, and do
                    not keep copies.
                  </li>
                  <li>
                    No denial of service, no volumetric scanning, no spam, and no
                    automated tooling that generates significant traffic.
                  </li>
                  <li>
                    No social engineering or phishing of our team or our customers,
                    and no physical attacks on our infrastructure.
                  </li>
                  <li>
                    Do not use a finding for anything beyond proving it exists, and
                    give us reasonable time to fix it before you publish. We will
                    coordinate disclosure with you, not against you.
                  </li>
                </ul>

                <h2 id="research">The vulnerability research programme</h2>
                <p>
                  The disclosure programme covers what we run. The research
                  programme covers what we ship. Marmot is MIT licensed and runs as one binary and a Postgres
                  database, which makes it an unusually convenient research target:
                  clone it, read the source, run it, fuzz it, and attack your own
                  instance as hard as you like. The{" "}
                  <Link to="/docs/quick-start">quick start</Link> gets you a catalog in a
                  couple of minutes.
                </p>
                <p>
                  Findings in the open source code get fixed in the open, with a
                  GitHub security advisory and a CVE where one is warranted, and
                  security fixes are ported to both the open source project and
                  Marmot Cloud, always. If your research raises a question we can
                  answer, whether something is intended behaviour, how a component
                  fits together, ask us at{" "}
                  <a href="mailto:security@marmotdata.io">security@marmotdata.io</a>{" "}
                  and we will actually answer.
                </p>

                <h2 id="safe-harbor">Safe harbour</h2>
                <p>
                  If you follow the rules on this page, your research is authorised.
                  We will not pursue legal action against you or report you to law
                  enforcement for good-faith research conducted under this programme,
                  and if a third party raises it, we will make clear that you were
                  acting with our authorisation. This does not permit breaking laws
                  that protect anyone other than us.
                </p>

                <h2 id="thanks">Thank you</h2>
                <p>
                  Marmot got better because people ran it and told us plainly where
                  it fell short. Security research is that same feedback loop with
                  higher stakes, and we are glad to have you in it. We are still
                  getting the company off the ground, so this is not a paid programme
                  yet, but good findings do not go unanswered: we send swag and
                  merch scaled to impact, and we credit reporters by name in the
                  advisory and the release notes if they want to be.
                </p>
              </article>
              <nav className={styles.toc} aria-labelledby="toc-title">
                <p className={styles.tocTitle} id="toc-title">
                  On this page
                </p>
                <ul className={styles.tocList}>
                  {sections.map((s) => (
                    <li key={s.id}>
                      <a href={`#${s.id}`}>{s.title}</a>
                    </li>
                  ))}
                </ul>
              </nav>
            </div>
          </div>
        </div>
      </div>
    </Layout>
  );
}
