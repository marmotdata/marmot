import React, { useEffect, useMemo, useRef, useState } from "react";
import SmartToy from "~icons/material-symbols/smart-toy-outline";
import Key from "~icons/material-symbols/key-outline";
import Check from "~icons/material-symbols/check";
import Block from "~icons/material-symbols/block";
import Help from "~icons/material-symbols/help-outline";
import Warning from "~icons/material-symbols/warning-outline";
import type { Player } from "@site/src/marketing/demo";
import { terrain } from "@site/src/marketing/terrain";

/*
 * What changes, as a map. Without Marmot the agent is on unmapped ground: it wanders from one
 * look-alike table to the next and walks straight into the customer table. With Marmot the same
 * ground is surveyed: one logged route to the certified table, and the personal data fenced off
 * as a restricted zone that a query is refused at. It plays like the hero, once and then again,
 * and the layer switch lets the visitor hold either world.
 */

interface Point {
  x: number;
  y: number;
}
interface Geometry {
  W: number;
  H: number;
  agent: Point;
  assets: Record<AssetId, Point>;
  zone: Point & { r: number };
  /** Where a refused query stops: on the zone's boundary. */
  stop: Point;
  route: string;
  spur: string;
}
type AssetId = "orders" | "v2" | "final" | "tmp" | "daily" | "customers";

const ASSETS: { id: AssetId; name: string; provider: string }[] = [
  { id: "orders", name: "orders", provider: "postgresql" },
  { id: "v2", name: "orders_v2", provider: "postgresql" },
  { id: "final", name: "orders_final", provider: "postgresql" },
  { id: "tmp", name: "revenue_tmp", provider: "snowflake" },
  { id: "daily", name: "orders_daily", provider: "snowflake" },
  { id: "customers", name: "customers", provider: "postgresql" },
];
// The order the agent wanders in when nothing tells it which table is which.
const WANDER: AssetId[] = ["orders", "v2", "final", "tmp", "customers"];

// A smooth line through points (Catmull-Rom as cubic Béziers).
function through(pts: Point[]): string {
  let d = `M${pts[0]!.x},${pts[0]!.y}`;
  for (let i = 0; i < pts.length - 1; i++) {
    const p0 = pts[i - 1] ?? pts[i]!;
    const p1 = pts[i]!;
    const p2 = pts[i + 1]!;
    const p3 = pts[i + 2] ?? p2;
    d += ` C${p1.x + (p2.x - p0.x) / 6},${p1.y + (p2.y - p0.y) / 6} ${p2.x - (p3.x - p1.x) / 6},${p2.y - (p3.y - p1.y) / 6} ${p2.x},${p2.y}`;
  }
  return d;
}

const WIDE: Geometry = {
  W: 1200,
  H: 480,
  agent: { x: 120, y: 250 },
  assets: {
    orders: { x: 400, y: 120 },
    v2: { x: 560, y: 175 },
    final: { x: 430, y: 270 },
    tmp: { x: 600, y: 320 },
    daily: { x: 1000, y: 140 },
    customers: { x: 940, y: 370 },
  },
  zone: { x: 940, y: 370, r: 96 },
  stop: { x: 846, y: 352 },
  route: "M186,240 C420,40 760,30 1000,140",
  spur: "M186,262 C420,450 700,420 846,352",
};
const TALL: Geometry = {
  W: 400,
  H: 780,
  agent: { x: 200, y: 50 },
  assets: {
    orders: { x: 80, y: 210 },
    v2: { x: 200, y: 270 },
    final: { x: 110, y: 340 },
    tmp: { x: 200, y: 420 },
    daily: { x: 310, y: 580 },
    customers: { x: 110, y: 660 },
  },
  zone: { x: 110, y: 660, r: 72 },
  stop: { x: 126, y: 590 },
  route: "M280,55 C390,170 380,450 310,580",
  spur: "M120,55 C20,280 30,480 126,590",
};

type Mode = "without" | "with";
type Leg = "wander" | "route" | "spur" | null;
interface State {
  mode: Mode;
  leg: Leg;
  /** The travelling agent's position, while it is on a leg. */
  dot: Point | null;
  /** How many look-alikes the wandering agent has reached. */
  visited: number;
  sweep: boolean;
  arrived: boolean;
  refused: boolean;
}
const START: State = { mode: "without", leg: null, dot: null, visited: 0, sweep: false, arrived: false, refused: false };
const MAPPED: State = { mode: "with", leg: null, dot: null, visited: 0, sweep: false, arrived: true, refused: true };
const UNMAPPED: State = { mode: "without", leg: null, dot: null, visited: WANDER.length, sweep: false, arrived: false, refused: false };

export default function BeforeAfter(): JSX.Element {
  const root = useRef<HTMLDivElement>(null);
  const switcher = useRef<HTMLElement>(null);
  const wanderRef = useRef<SVGPathElement>(null);
  const routeRef = useRef<SVGPathElement>(null);
  const spurRef = useRef<SVGPathElement>(null);
  const playerRef = useRef<Player>();
  // The story opens on unmapped ground and waits there until the map scrolls into view.
  const [s, setS] = useState<State>(START);
  const [tall, setTall] = useState(false);
  const g = tall ? TALL : WIDE;

  const wander = useMemo(() => through([g.agent, ...WANDER.map((id) => g.assets[id])]), [g]);
  const ground = useMemo(() => terrain([g.agent, ...Object.values(g.assets)], g.W, g.H, { seed: 11, spread: tall ? 180 : 260 }), [g, tall]);
  const pct = (p: Point) => ({ left: `${(p.x / g.W) * 100}%`, top: `${(p.y / g.H) * 100}%` });

  useEffect(() => {
    const narrow = matchMedia("(max-width: 720px)");
    const fit = () => setTall(narrow.matches);
    fit();
    narrow.addEventListener("change", fit);
    void import("@marmotdata/elements/segmented-control.js");
    void import("@marmotdata/icons/define.js");

    // The switch holds whichever world the visitor picks, and stops the story playing over it.
    const el = switcher.current;
    const onChange = (e: Event) => {
      const mode = (e.currentTarget as HTMLInputElement).value as Mode;
      if (playerRef.current && !playerRef.current.paused) playerRef.current.toggle();
      setS(mode === "with" ? MAPPED : UNMAPPED);
    };
    el?.addEventListener("change", onChange);

    let p: Player | undefined;
    let cancelled = false;
    // Under reduced motion there is no story to play, so the map shows how it ends.
    if (matchMedia("(prefers-reduced-motion: reduce)").matches) setS(MAPPED);
    else {
      void import("@site/src/marketing/demo").then(({ play }) => {
        if (cancelled || !root.current) return;
        let cur = { ...START };
        const set = (patch: Partial<State>) => setS((cur = { ...cur, ...patch }));
        const along = (path: SVGPathElement | null, t: number) => {
          if (!path) return null;
          const at = path.getPointAtLength(path.getTotalLength() * t);
          return { x: at.x, y: at.y };
        };
        p = playerRef.current = play(root.current, {
          reset: () => set({ ...START }),
          async run(demo) {
            await demo.wait(700);
            // Unmapped: the agent tries one look-alike after another, then the customer table.
            set({ leg: "wander" });
            await demo.animate(6000, (t) => set({ dot: along(wanderRef.current, t), visited: Math.min(WANDER.length, Math.floor(t * WANDER.length + 0.05)) }));
            await demo.wait(1600);
            // Marmot surveys the ground: the line passes, the contours and the zone appear.
            set({ leg: null, dot: null, mode: "with", sweep: true });
            await demo.wait(1700);
            set({ sweep: false });
            set({ leg: "route" });
            await demo.animate(2400, (t) => set({ dot: along(routeRef.current, t) }));
            set({ arrived: true, dot: null });
            await demo.wait(900);
            set({ leg: "spur" });
            await demo.animate(1500, (t) => set({ dot: along(spurRef.current, t) }));
            set({ refused: true, dot: null });
            await demo.wait(5200);
          },
          rest: 1200,
        });
      });
    }
    return () => {
      cancelled = true;
      narrow.removeEventListener("change", fit);
      el?.removeEventListener("change", onChange);
      p?.destroy();
    };
  }, []);

  const mapped = s.mode === "with";
  const reached = (id: AssetId) => WANDER.indexOf(id) > -1 && WANDER.indexOf(id) < s.visited;

  return (
    <div className={`mkt-survey is-${s.mode}${tall ? " is-tall" : ""}`} ref={root}>
      <div className="mkt-survey__layers">
        <mm-segmented-control ref={switcher} label="Map layer" hide-label value={s.mode}>
          <mm-segment value="without">Without Marmot</mm-segment>
          <mm-segment value="with">With Marmot</mm-segment>
        </mm-segmented-control>
      </div>

      <div className="mkt-survey__map">
        <svg viewBox={`0 0 ${g.W} ${g.H}`} className="mkt-survey__svg" aria-hidden="true">
          <defs>
            <pattern id="mkt-restricted" width="8" height="8" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
              <line x1="0" y1="0" x2="0" y2="8" className="mkt-survey__hatch" />
            </pattern>
          </defs>
          <g className="mkt-survey__ground">
            {ground.map((c, i) => (
              <path key={i} d={c.d} pathLength={1} className={c.index ? "is-index" : undefined} style={{ transitionDelay: `${(i * 0.05).toFixed(2)}s` }} />
            ))}
          </g>
          <g className="mkt-survey__zone">
            <circle cx={g.zone.x} cy={g.zone.y} r={g.zone.r} fill="url(#mkt-restricted)" />
            <circle cx={g.zone.x} cy={g.zone.y} r={g.zone.r} className="mkt-survey__fence" />
          </g>
          <path ref={wanderRef} d={wander} className="mkt-survey__wander" />
          <path ref={routeRef} d={g.route} pathLength={1} className={`mkt-survey__route${s.leg === "route" || s.arrived ? " is-drawn" : ""}`} />
          <path ref={spurRef} d={g.spur} className={`mkt-survey__spur${s.leg === "spur" || s.refused ? " is-on" : ""}`} />
          {s.refused && <circle cx={g.stop.x} cy={g.stop.y} r={5} className="mkt-survey__stop" />}
          <line x1={0} y1={0} x2={0} y2={g.H} className={`mkt-survey__sweep${s.sweep ? " is-on" : ""}`} style={{ "--mkt-sweep": `${g.W}px` } as React.CSSProperties} />
          {s.dot && <circle cx={s.dot.x} cy={s.dot.y} r={6} className="mkt-survey__dot" />}
        </svg>

        <div className="mkt-survey__agent" style={pct(g.agent)}>
          <span className="mkt-survey__node">
            <SmartToy aria-hidden="true" />
            finance-agent
          </span>
          <span className="mkt-survey__badge mkt-survey__badge--warn mkt-survey__only-without">
            <Key aria-hidden="true" />
            Read-only database password
          </span>
          <span className="mkt-survey__badge mkt-survey__only-with">Over MCP</span>
        </div>

        {ASSETS.map((a) => {
          const at = g.assets[a.id];
          const state = mapped
            ? a.id === "daily"
              ? s.arrived
                ? "is-found"
                : ""
              : a.id === "customers"
                ? s.refused
                  ? "is-refused"
                  : ""
                : "is-quiet"
            : reached(a.id)
              ? a.id === "customers"
                ? "is-refused"
                : "is-traced"
              : "";
          const flip = at.x > g.W * (tall ? 0.72 : 0.8);
          return (
            <div key={a.id} className={`mkt-maphero__asset ${state}${flip ? " is-flipped" : ""}`} style={pct(at)}>
              <span className="mkt-maphero__pin" />
              <span className="mkt-maphero__label">
                <mm-provider-logo provider={a.provider} decorative></mm-provider-logo>
                {a.name}
              </span>
              {!mapped && reached(a.id) && a.id !== "customers" && (
                <span className="mkt-maphero__note mkt-maphero__note--warn">
                  <Help aria-hidden="true" />
                  Is this revenue?
                </span>
              )}
              {!mapped && reached(a.id) && a.id === "customers" && (
                <span className="mkt-maphero__note mkt-maphero__note--warn">
                  <Warning aria-hidden="true" />
                  Emails read
                </span>
              )}
              {mapped && a.id === "daily" && s.arrived && (
                <span className="mkt-maphero__note">
                  <Check aria-hidden="true" />
                  Certified · 7 rows
                </span>
              )}
              {mapped && a.id === "customers" && s.refused && (
                <span className="mkt-maphero__note mkt-maphero__note--refused">
                  <Block aria-hidden="true" />
                  Refused
                </span>
              )}
            </div>
          );
        })}
        <span className="mkt-survey__zone-label mkt-survey__only-with" style={pct({ x: g.zone.x, y: tall ? g.zone.y + g.zone.r + 26 : g.zone.y - g.zone.r - 30 })}>
          Personal data · needs a grant
        </span>

      </div>
      {/* The map itself is drawn for the eye; this says what it shows, for a screen reader. */}
      <p className="mm-visually-hidden" aria-live="polite">
        {mapped
          ? "With Marmot: the agent takes one logged route to the certified orders_daily table, and its query on the customer table is refused."
          : "Without Marmot: the agent guesses between look-alike order tables with a database password, and ends up reading customer emails."}
      </p>
    </div>
  );
}
