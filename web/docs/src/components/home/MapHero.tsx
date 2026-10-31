import React, { useEffect, useMemo, useRef, useState } from "react";
import { contourDensity } from "d3-contour";
import { geoPath } from "d3-geo";
import PauseIcon from "~icons/material-symbols/pause";
import PlayIcon from "~icons/material-symbols/play-arrow";
import Check from "~icons/material-symbols/check";
import Block from "~icons/material-symbols/block";
import SmartToy from "~icons/material-symbols/smart-toy-outline";
import Send from "~icons/material-symbols/arrow-upward";
import type { Player } from "@site/src/marketing/demo";

/*
 * The hero: a sample organisation's data as a surveyed map. The ground rises where data
 * gathers, so the contours are drawn from the assets themselves; lineage runs between them as
 * tracks. Over it, one agent question plays out end to end, the thing Marmot does that a
 * catalog alone cannot: it finds the right table, checks the agent may read it, runs the query,
 * refuses the one it may not, and keeps a record of both.
 */

const W = 1000;
const H = 640;

interface Asset {
  id: string;
  name: string;
  provider: string;
  kind: string;
  x: number;
  y: number;
}

// Sources at the top left, models in the middle, dashboards to the right; the lower left is open
// ground for the chat window to float over.
const ASSETS: Asset[] = [
  { id: "orders", name: "orders", provider: "postgresql", kind: "Table", x: 130, y: 150 },
  { id: "events", name: "shop.orders.v2", provider: "kafka", kind: "Topic", x: 320, y: 78 },
  { id: "customers", name: "customers", provider: "postgresql", kind: "Table", x: 210, y: 222 },
  { id: "stg", name: "stg_orders", provider: "dbt", kind: "Model", x: 450, y: 200 },
  { id: "daily", name: "orders_daily", provider: "snowflake", kind: "Table", x: 620, y: 268 },
  { id: "dim", name: "customers_dim", provider: "snowflake", kind: "Table", x: 470, y: 380 },
  { id: "revenue", name: "Revenue overview", provider: "looker", kind: "Dashboard", x: 860, y: 110 },
  { id: "health", name: "Customer health", provider: "looker", kind: "Dashboard", x: 850, y: 490 },
];
const at = (id: string) => ASSETS.find((a) => a.id === id)!;


const TRACKS: [string, string][] = [
  ["orders", "events"],
  ["orders", "stg"],
  ["stg", "daily"],
  ["customers", "dim"],
  ["daily", "revenue"],
  ["daily", "health"],
  ["dim", "health"],
];
// The tracks the agent reads: where revenue comes from.
const UPSTREAM = new Set(["orders>stg", "stg>daily", "daily>revenue"]);

const track = ([a, b]: [string, string]) => {
  const p = at(a);
  const q = at(b);
  const mx = (p.x + q.x) / 2;
  return `M${p.x},${p.y} C${mx},${p.y} ${mx},${q.y} ${q.x},${q.y}`;
};

// Deterministic ground: each asset raises a hill, and a scatter of lighter points around it
// keeps the terrain from reading as a set of rings.
function groundPoints(): [number, number, number][] {
  let seed = 7;
  const rand = () => ((seed = (seed * 16807) % 2147483647) - 1) / 2147483646;
  const pts: [number, number, number][] = [];
  for (const a of ASSETS) {
    pts.push([a.x, a.y, 6]);
    for (let i = 0; i < 14; i++) pts.push([a.x + (rand() - 0.5) * 260, a.y + (rand() - 0.5) * 220, 1]);
  }
  for (let i = 0; i < 70; i++) pts.push([rand() * W, rand() * H, 0.6]);
  return pts;
}

// One agent question, step by step, in plain words.
const QUESTION = "What was revenue last week, and can I trust the number?";
type Step = { text: string; detail: string; refused?: boolean };
const STEPS: Step[] = [
  { text: "Searched the catalog", detail: "orders_daily, certified, owned by Analytics Engineering" },
  { text: "Read its lineage", detail: "Built by dbt from shop.public.orders" },
  { text: "Checked its access", detail: "svc-finance may read orders_daily" },
  { text: "Ran a query on orders_daily", detail: "7 rows in 412 ms, on record" },
  { text: "Asked for customer emails", detail: "Refused: no grant on customers.email", refused: true },
];
const ANSWER = "Revenue last week was £413,180 across 8,615 orders, from the certified orders_daily.";

interface State {
  question: string;
  step: number; // steps shown
  done: number; // steps finished
  sweep: boolean;
  /** The question has left the composer and sits in the conversation. */
  sent: boolean;
  answered: boolean;
}
const START: State = { question: "", step: 0, done: 0, sweep: false, sent: false, answered: false };
const END: State = { question: QUESTION, step: STEPS.length, done: STEPS.length, sweep: false, sent: true, answered: true };

export default function MapHero(): JSX.Element {
  const root = useRef<HTMLDivElement>(null);
  // The finished picture renders first, so the page is whole without script or motion.
  const [s, setS] = useState<State>(END);
  const [player, setPlayer] = useState<Player>();
  const [paused, setPaused] = useState(false);
  const [still, setStill] = useState(true);
  const [drawn, setDrawn] = useState(false);

  const contours = useMemo(() => {
    const density = contourDensity<[number, number, number]>()
      .x((d) => d[0])
      .y((d) => d[1])
      .weight((d) => d[2])
      .size([W, H])
      .cellSize(4)
      .bandwidth(46)
      .thresholds(16)(groundPoints());
    const path = geoPath();
    return density.map((c, i) => ({ d: path(c) ?? "", index: i % 5 === 4 }));
  }, []);

  useEffect(() => {
    const reduced = matchMedia("(prefers-reduced-motion: reduce)").matches;
    setStill(reduced);
    setDrawn(true);
    if (reduced || !root.current) return;
    let p: Player | undefined;
    let cancelled = false;
    void import("@marmotdata/icons/define.js");
    void import("@site/src/marketing/demo").then(({ play, type }) => {
      if (cancelled || !root.current) return;
      let cur = { ...START };
      const set = (patch: Partial<State>) => setS((cur = { ...cur, ...patch }));
      p = play(root.current, {
        reset: () => set({ ...START }),
        async run(demo) {
          // The survey draws in before the question arrives.
          await demo.wait(1800);
          await type(demo, QUESTION, (question) => set({ question }), 28);
          await demo.wait(450);
          set({ sent: true });
          for (let i = 0; i < STEPS.length; i++) {
            await demo.wait(500);
            set({ step: i + 1, sweep: i === 0 });
            await demo.wait(i === 0 ? 1600 : 1100);
            set({ done: i + 1, sweep: false });
          }
          await demo.wait(500);
          set({ answered: true });
          await demo.wait(6000);
        },
        rest: 1500,
      });
      p.onChange = setPaused;
      setPlayer(p);
    });
    return () => {
      cancelled = true;
      p?.destroy();
    };
  }, []);

  // What the map shows at each point in the story.
  const found = s.done >= 1;
  const tracing = s.step >= 2;
  const allowed = s.done >= 3;
  const queried = s.done >= 4;
  const refusing = s.step >= 5;

  return (
    <div className="mkt-maphero" ref={root}>
      <div className={`mkt-maphero__map${drawn ? " is-drawn" : ""}`}>
        <svg viewBox={`0 0 ${W} ${H}`} className="mkt-maphero__svg" aria-hidden="true">
          <g className="mkt-maphero__ground">
            {contours.map((c, i) => (
              <path
                key={i}
                d={c.d}
                pathLength={1}
                className={c.index ? "is-index" : undefined}
                style={{ animationDelay: `${(i * 0.06).toFixed(2)}s` }}
              />
            ))}
          </g>
          <g className="mkt-maphero__tracks">
            {TRACKS.map((t) => {
              const key = `${t[0]}>${t[1]}`;
              return <path key={key} d={track(t)} className={tracing && UPSTREAM.has(key) ? "is-traced" : undefined} />;
            })}
          </g>
          {/* The survey line that passes over the map while the agent searches. */}
          <line x1={0} y1={0} x2={0} y2={H} className={`mkt-maphero__sweep${s.sweep ? " is-on" : ""}`} />
        </svg>
        {ASSETS.map((a) => {
          const state =
            a.id === "daily" && found ? "is-found" : a.id === "customers" && refusing ? "is-refused" : tracing && ["orders", "stg", "revenue"].includes(a.id) ? "is-traced" : "";
          return (
            <div key={a.id} className={`mkt-maphero__asset ${state}${a.x > 700 ? " is-flipped" : ""}`} style={{ left: `${(a.x / W) * 100}%`, top: `${(a.y / H) * 100}%` }}>
              <span className="mkt-maphero__pin" />
              <span className="mkt-maphero__label">
                <mm-provider-logo provider={a.provider} decorative></mm-provider-logo>
                {a.name}
              </span>
              {a.id === "daily" && allowed && (
                <span className="mkt-maphero__note">
                  <Check aria-hidden="true" />
                  {queried ? "7 rows" : "Access allowed"}
                </span>
              )}
              {a.id === "customers" && refusing && (
                <span className="mkt-maphero__note mkt-maphero__note--refused">
                  <Block aria-hidden="true" />
                  Refused
                </span>
              )}
            </div>
          );
        })}
      </div>

      {/* The agent's side of it, as a chat: the question is typed and sent, the steps it took show
          as tool use, and the answer follows. A fixed window, so it never grows over the map; older
          lines fade out at the top as new ones arrive. */}
      <section className="mkt-chat" aria-label="An agent's question, answered through Marmot">
        <header className="mkt-chat__head">
          <span className="mkt-chat__who">
            <SmartToy aria-hidden="true" />
            finance-agent
            <span className="mkt-chat__via">over MCP · sample data</span>
          </span>
          {!still && player && (
            <button
              type="button"
              className="mm-btn mm-btn--ghost mm-btn--sm mm-btn--icon"
              onClick={() => player.toggle()}
              aria-label={paused ? "Play the demo" : "Pause the demo"}
            >
              {paused ? <PlayIcon aria-hidden="true" /> : <PauseIcon aria-hidden="true" />}
            </button>
          )}
        </header>
        <div className="mkt-chat__log">
          {s.sent && <p className="mkt-chat__mine">{QUESTION}</p>}
          {s.step > 0 && (
            <ol className="mkt-chat__tools">
              {STEPS.slice(0, s.step).map((step, i) => {
                const done = i < s.done;
                return (
                  <li key={step.text} className={step.refused && done ? "is-refused" : done ? "is-done" : "is-running"}>
                    {step.refused && done ? <Block aria-hidden="true" /> : <Check aria-hidden="true" />}
                    <span>
                      {step.text}
                      {done && <span className="mkt-chat__detail">{step.detail}</span>}
                    </span>
                  </li>
                );
              })}
            </ol>
          )}
          {s.answered && <p className="mkt-chat__answer">{ANSWER}</p>}
        </div>
        <div className="mkt-chat__composer" aria-hidden="true">
          <span className={s.question && !s.sent ? "is-typing" : undefined}>{s.question && !s.sent ? s.question : "Ask about your data"}</span>
          <Send />
        </div>
      </section>
    </div>
  );
}
