import React, { useEffect, useRef, useState } from "react";
import { html, render, svg, type TemplateResult } from "lit";
import MenuBook from "~icons/material-symbols/menu-book-outline";
import Policy from "~icons/material-symbols/policy-outline";
import History from "~icons/material-symbols/history";
import Hub from "~icons/material-symbols/hub-outline";

/*
 * The problem, then what changes, kept to one screen: the statement with its answer in a second,
 * quieter sentence and the evidence under it, one picture beside it, and four short points in a
 * row underneath. Each point has its own picture, made from the product's real elements (the
 * schema table, a refused query, the query audit, agents in lineage) at the app's size and scaled
 * to fit. While the section is on screen the points take turns: the current one is in ink with
 * its hairline filling, and the picture changes to match and plays its few frames. Choosing a
 * point shows it and carries on from there. Under reduced motion nothing advances on its own.
 */

const STEP = 4600;

interface Gain<S> {
  title: string;
  text: string;
  /** The width the elements are laid out at, before scaling to the column. */
  width: number;
  /** Never scale below this; what no longer fits is cropped from the start, keeping the end. */
  minScale?: number;
  label: string;
  /** Frames from the start of the gain's turn; the last is how it rests. */
  frames: { at: number; s: S }[];
  view: (s: S) => TemplateResult;
}
const gain = <S,>(g: Gain<S>) => g as unknown as Gain<unknown>;

const terms = gain<{ term: boolean; certified: boolean }>({
  title: "Definitions.",
  text: "Glossary terms and certified tables, read before every query.",
  width: 620,
  label:
    "The schema of orders_daily, where net_revenue_gbp is given the glossary term Net revenue and then certified.",
  frames: [
    { at: 0, s: { term: false, certified: false } },
    { at: 900, s: { term: true, certified: false } },
    { at: 2100, s: { term: true, certified: true } },
  ],
  view: (s) =>
    html`<div class="mkt-pic__stack">
      <div
        class="mm-cluster"
        style="--mm-gap: var(--space-2); min-block-size: 24px"
      >
        <mm-text variant="caption" tone="secondary"
          >Glossary terms for net_revenue_gbp</mm-text
        >
        ${s.term ? html`<mm-tag href="#glossary/net-revenue">Net revenue</mm-tag>` : ""}
      </div>
      <mm-schema-table
        label="Schema of orders_daily"
        density="compact"
        hide-toolbar
        .columns=${[
          {
            name: "gross_gbp",
            type: "decimal(14,2)",
            description:
              "Order totals including VAT before refunds and discounts.",
          },
          {
            name: "refunds_gbp",
            type: "decimal(14,2)",
            description: "Refunds settled against orders from this day.",
          },
          {
            name: "net_revenue_gbp",
            type: "decimal(14,2)",
            description:
              "Gross less discounts, refunds and VAT. The figure finance reports.",
            glossaryTerms: ["Net revenue"],
            certified: s.certified,
          },
        ]}
      ></mm-schema-table>
    </div>`,
});

const REFUSED_SQL =
  "select email, signup_date\nfrom analytics.marts.customers_dim\nwhere signup_date >= current_date - 7;";
const checked = gain<{ refused: boolean }>({
  title: "Access.",
  text: "Every query checked against what that agent may read.",
  width: 520,
  label:
    "growth-agent queries customer emails, and Marmot refuses the query because of an access policy.",
  frames: [
    { at: 0, s: { refused: false } },
    { at: 1300, s: { refused: true } },
  ],
  view: (s) =>
    html`<div class="mkt-pic__stack">
      <mm-code-sample
        language="sql"
        no-copy
        label="Query from growth-agent"
        .code=${REFUSED_SQL}
      ></mm-code-sample>
      ${
        s.refused
          ? html`<mm-callout
              tone="danger"
              heading="Marmot refused this query because of an access policy."
              >growth-agent does not have asset:query on
              customers_dim.</mm-callout
            >`
          : html`<div class="mkt-pic__hold"></div>`
      }
    </div>`,
});

const rows = [
  {
    id: "a",
    at: "2026-10-27T09:29:12Z",
    actor: "growth-agent",
    sa: "svc-growth-agent",
    source: "snowflake-finance",
    provider: "snowflake",
    sql: REFUSED_SQL,
    rows: undefined,
    outcome: "denied",
    reason: "No asset:query on customers_dim",
  },
  {
    id: "b",
    at: "2026-10-27T09:27:40Z",
    actor: "cli-agent",
    sa: "svc-cli-agent",
    source: "warehouse",
    provider: "postgresql",
    sql: "select * from stg_orders limit 50;",
    rows: 50,
    outcome: "allowed",
    reason: "",
  },
  {
    id: "c",
    at: "2026-10-27T09:26:03Z",
    actor: "finance-agent",
    sa: "svc-finance-agent",
    source: "snowflake-finance",
    provider: "snowflake",
    sql: "select order_date, net_revenue_gbp from orders_daily;",
    rows: 7,
    outcome: "allowed",
    reason: "",
  },
];
const record = gain<{ shown: number }>({
  title: "Audit.",
  text: "Who asked, what ran and whether it was allowed.",
  width: 1040,
  minScale: 0.78,
  label: "The query audit fills with three agent queries, the newest refused.",
  frames: [
    { at: 0, s: { shown: 0 } },
    { at: 500, s: { shown: 1 } },
    { at: 1200, s: { shown: 2 } },
    { at: 1900, s: { shown: 3 } },
  ],
  view: (s) =>
    html`<mm-audit-log label="Query audit log" class="mkt-pic__audit">
      ${rows
        .slice(rows.length - s.shown)
        .map(
          (r) =>
            html`<mm-audit-row
              time=${r.at}
              actor=${r.actor}
              actor-kind="agent"
              service-account=${r.sa}
              client="mcp"
              source=${r.source}
              provider=${r.provider}
              .query=${r.sql}
              .rows=${r.rows}
              outcome=${r.outcome}
              reason=${r.reason}
            ></mm-audit-row>`,
        )}
    </mm-audit-log>`,
});

const agents = ["finance-agent", "support-triage-agent", "cli-agent"];
const source = gain<{ wired: number }>({
  title: "One source.",
  text: "Agents from any vendor read the same facts over MCP.",
  width: 720,
  label: "Three agents, each reading orders_daily through Marmot.",
  frames: [
    { at: 0, s: { wired: 0 } },
    { at: 600, s: { wired: 1 } },
    { at: 1100, s: { wired: 2 } },
    { at: 1600, s: { wired: 3 } },
  ],
  view: (s) =>
    html`<div class="mkt-pic__hub">
      <div class="mkt-pic__agents">
        ${agents.map((a) => html`<mm-lineage-node name=${a} type="Agent" provider="agent"></mm-lineage-node>`)}
      </div>
      <div class="mkt-pic__wires">
        <svg
          viewBox="0 0 100 300"
          preserveAspectRatio="none"
          aria-hidden="true"
        >
          ${[50, 150, 250].map(
            (y, i) =>
              svg`<path d="M0,${y} C50,${y} 50,150 100,150" class=${i < s.wired ? "is-on" : ""}></path>`,
          )}
        </svg>
      </div>
      <mm-lineage-node
        name="orders_daily"
        type="Table"
        provider="snowflake"
        ?selected=${s.wired === 3}
      ></mm-lineage-node>
    </div>`,
});

const gains = [terms, checked, record, source];
const icons = [MenuBook, Policy, History, Hub];
const last = <S,>(g: Gain<S>) => g.frames[g.frames.length - 1]!.s;

/** Real elements at the app's size, scaled to fit the column and drawn with Lit. */
function Picture({
  g,
  playing,
  shown,
}: {
  g: Gain<unknown>;
  playing: boolean;
  shown: boolean;
}): JSX.Element {
  const box = useRef<HTMLDivElement>(null);
  const stage = useRef<HTMLDivElement>(null);
  const [ready, setReady] = useState(false);
  const [s, setS] = useState(() => last(g));
  const [fit, setFit] = useState({ k: 1, h: 0, crop: 0 });

  useEffect(() => {
    let cancelled = false;
    void import("@site/src/marketing/screens/elements").then(
      ({ defineScreenElements }) =>
        defineScreenElements().then(() => !cancelled && setReady(true)),
    );
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (!playing) {
      setS(last(g));
      return;
    }
    const timers = g.frames.map((f) =>
      window.setTimeout(() => setS(f.s), f.at),
    );
    return () => timers.forEach((t) => window.clearTimeout(t));
  }, [playing, g]);

  useEffect(() => {
    if (ready && stage.current) render(g.view(s), stage.current);
  }, [ready, s, g]);

  useEffect(() => {
    if (!box.current || !stage.current) return;
    let frame = 0;
    const measure = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => {
        const b = box.current;
        const st = stage.current;
        if (!b || !st) return;
        const cs = getComputedStyle(b);
        const w =
          b.clientWidth -
          parseFloat(cs.paddingInlineStart) -
          parseFloat(cs.paddingInlineEnd);
        const h =
          b.clientHeight -
          parseFloat(cs.paddingBlockStart) -
          parseFloat(cs.paddingBlockEnd);
        const natural = st.offsetHeight;
        const k = Math.max(
          g.minScale ?? 0,
          Math.min(1, w / g.width, natural ? h / natural : 1),
        );
        setFit({ k, h: natural * k, crop: Math.max(0, g.width * k - w) });
      });
    };
    const observer = new ResizeObserver(measure);
    observer.observe(box.current);
    observer.observe(stage.current);
    return () => {
      cancelAnimationFrame(frame);
      observer.disconnect();
    };
  }, [g]);

  return (
    <figure
      className={`mkt-pic${shown ? " is-shown" : ""}`}
      ref={box}
      aria-label={g.label}
      aria-hidden={!shown}
    >
      <div
        className="mkt-pic__fit"
        style={{ inlineSize: g.width * fit.k - fit.crop, blockSize: fit.h }}
      >
        <div
          className="mkt-pic__stage"
          ref={stage}
          inert
          aria-hidden="true"
          style={{
            inlineSize: g.width,
            transform: `translateX(${-fit.crop}px) scale(${fit.k})`,
          }}
        />
      </div>
    </figure>
  );
}

export default function Problem(): JSX.Element {
  const root = useRef<HTMLElement>(null);
  // -1 until the section is on screen: the first picture shows, finished, and nothing plays.
  const [active, setActive] = useState(-1);
  const [visible, setVisible] = useState(false);
  const [reduced, setReduced] = useState(false);

  useEffect(() => {
    const el = root.current;
    if (!el) return;
    setReduced(window.matchMedia("(prefers-reduced-motion: reduce)").matches);
    const observer = new IntersectionObserver(
      ([entry]) => setVisible(!!entry?.isIntersecting),
      { threshold: 0.3 },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  // Advances while on screen; choosing a point resets the clock, because `active` changes.
  useEffect(() => {
    if (!visible || reduced) return;
    if (active < 0) {
      setActive(0);
      return;
    }
    const t = window.setTimeout(
      () => setActive((i) => (i + 1) % gains.length),
      STEP,
    );
    return () => window.clearTimeout(t);
  }, [visible, reduced, active]);

  const shown = Math.max(0, active);
  return (
    <section
      ref={root}
      className="mkt-problem"
      aria-labelledby="problem"
      style={{ "--mkt-step": `${STEP}ms` } as React.CSSProperties}
    >
      <div className="mm-container">
        <div className="mkt-problem__top">
          <div className="mkt-problem__head">
            <h2 className="mkt-problem__title" id="problem">
              Agents are only as good as what they know.{" "}
              <span>
                Marmot gives them what your company knows, under your rules.
              </span>
            </h2>
            <p className="mkt-problem__lede">
              In{" "}
              <a
                className="mm-link"
                href="https://www.deloitte.com/us/en/what-we-do/capabilities/applied-artificial-intelligence/content/state-of-ai-in-the-enterprise.html"
              >
                Deloitte’s 2026 survey
              </a>{" "}
              of 3,235 senior leaders, 74% of companies planned to deploy AI
              agents within two years, but only 21% had a mature model for
              governing them.
            </p>
          </div>
          <div className="mkt-pics">
            <mm-topography
              seed="19"
              density="sparse"
              fade="edges"
            ></mm-topography>
            {gains.map((g, i) => (
              <Picture
                key={g.title}
                g={g}
                shown={i === shown}
                playing={i === active && !reduced}
              />
            ))}
          </div>
        </div>
        <ul className="mkt-gains">
          {gains.map((g, i) => {
            const Icon = icons[i]!;
            return (
              <li
                key={g.title}
                className={i === shown ? "is-active" : undefined}
              >
                <button
                  type="button"
                  aria-pressed={i === shown}
                  onClick={() => setActive(i)}
                >
                  <Icon aria-hidden="true" />
                  <span className="mkt-gains__rule" aria-hidden="true" />
                  <span className="mkt-gains__text">
                    <strong>{g.title}</strong> {g.text}
                  </span>
                </button>
              </li>
            );
          })}
        </ul>
      </div>
    </section>
  );
}
