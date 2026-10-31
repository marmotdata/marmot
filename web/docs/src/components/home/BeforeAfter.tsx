import React, { useEffect, useMemo, useRef, useState } from "react";
import SmartToy from "~icons/material-symbols/smart-toy-outline";
import Key from "~icons/material-symbols/key-outline";
import Check from "~icons/material-symbols/check";
import Block from "~icons/material-symbols/block";
import Help from "~icons/material-symbols/help-outline";
import Warning from "~icons/material-symbols/warning-outline";
import Route from "~icons/material-symbols/route-outline";
import type { Player } from "@site/src/marketing/demo";
import Beats from "@site/src/components/home/Beats";
import { terrain } from "@site/src/marketing/terrain";

/*
 * What changes, as two map sheets of the same ground side by side. Without Marmot the ground is
 * unmapped: the agent wanders from one look-alike table to the next and walks straight into the
 * customer table. With Marmot the ground is surveyed: one logged route to the certified table, and
 * the personal data fenced off as a restricted zone that a query is refused at. Both sheets are
 * on screen throughout, so the difference shows before anything plays; the story then plays one
 * sheet and then the other, captioned above them like the chapters' points.
 */

interface Point {
  x: number;
  y: number;
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

// One sheet's ground. Both sheets share it, so the only difference between them is what the agent
// does. A phone gets a tall sheet, with the agent at the top and the tables below it.
interface Geometry {
  W: number;
  H: number;
  agent: Point;
  at: Record<AssetId, Point>;
  zone: Point & { r: number };
  /** Where a refused query stops: on the zone's boundary. */
  stop: Point;
  route: string;
  spur: string;
  /** Where the zone's label sits. */
  zoneLabel: Point;
}
const WIDE: Geometry = {
  W: 600,
  H: 420,
  agent: { x: 100, y: 200 },
  at: {
    orders: { x: 210, y: 90 },
    v2: { x: 320, y: 140 },
    final: { x: 210, y: 225 },
    tmp: { x: 330, y: 280 },
    daily: { x: 500, y: 70 },
    customers: { x: 470, y: 330 },
  },
  zone: { x: 470, y: 330, r: 62 },
  stop: { x: 409, y: 318 },
  route: "M166,190 C230,-50 420,-10 500,70",
  spur: "M166,212 C240,390 330,380 409,318",
  zoneLabel: { x: 470, y: 238 },
};
const TALL: Geometry = {
  W: 340,
  H: 520,
  agent: { x: 170, y: 40 },
  at: {
    orders: { x: 90, y: 150 },
    v2: { x: 190, y: 200 },
    final: { x: 90, y: 270 },
    tmp: { x: 190, y: 330 },
    daily: { x: 300, y: 130 },
    customers: { x: 200, y: 440 },
  },
  zone: { x: 200, y: 440, r: 50 },
  stop: { x: 152, y: 428 },
  route: "M235,45 C300,60 305,90 300,130",
  spur: "M105,50 C0,200 10,400 152,428",
  zoneLabel: { x: 200, y: 496 },
};

// How long each part of the story takes, so the captions can tell where one sheet's turn ends.
const T = { start: 700, wander: 4500, lost: 1200, pause: 600, route: 2400, arrived: 900, spur: 1500, end: 5200 };
const WITHOUT_ENDS = T.start + T.wander + T.lost;
const BOUNDS = [0, WITHOUT_ENDS, WITHOUT_ENDS + T.pause + T.route + T.arrived + T.spur + T.end];
const HALVES = [
  { Icon: Key, text: "Without Marmot", detail: "It has a database password and guesses which table holds revenue." },
  { Icon: Route, text: "With Marmot", detail: "It finds the certified table, and its query on customer emails is refused." },
];

type Leg = "wander" | "route" | "spur" | null;
interface State {
  leg: Leg;
  /** The travelling agent's position, while it is on a leg. */
  dot: Point | null;
  /** How many look-alikes the wandering agent has reached. */
  visited: number;
  arrived: boolean;
  refused: boolean;
}
const START: State = { leg: null, dot: null, visited: 0, arrived: false, refused: false };

const pct = (g: Geometry, p: Point) => ({ left: `${(p.x / g.W) * 100}%`, top: `${(p.y / g.H) * 100}%` });

function Asset({ g, id, name, provider, state, children }: { g: Geometry; id: AssetId; name: string; provider: string; state: string; children?: React.ReactNode }): JSX.Element {
  const at = g.at[id];
  return (
    <div className={`mkt-maphero__asset ${state}${at.x > g.W * 0.8 ? " is-flipped" : ""}`} style={pct(g, at)}>
      <span className="mkt-maphero__pin" />
      <span className="mkt-maphero__label">
        <mm-provider-logo provider={provider} decorative></mm-provider-logo>
        {name}
      </span>
      {children}
    </div>
  );
}

function Agent({ g, children }: { g: Geometry; children: React.ReactNode }): JSX.Element {
  return (
    <div className="mkt-survey__agent" style={pct(g, g.agent)}>
      <span className="mkt-survey__node">
        <SmartToy aria-hidden="true" />
        finance-agent
      </span>
      {children}
    </div>
  );
}

export default function BeforeAfter(): JSX.Element {
  const root = useRef<HTMLDivElement>(null);
  const wanderRef = useRef<SVGPathElement>(null);
  const routeRef = useRef<SVGPathElement>(null);
  const spurRef = useRef<SVGPathElement>(null);
  const withSheet = useRef<HTMLDivElement>(null);
  const [player, setPlayer] = useState<Player>();
  // The story opens with both sheets still and waits there until they scroll into view.
  const [s, setS] = useState<State>(START);
  const [tall, setTall] = useState(false);
  const g = tall ? TALL : WIDE;
  const wander = useMemo(() => through([g.agent, ...WANDER.map((id) => g.at[id])]), [g]);
  const ground = useMemo(() => terrain([g.agent, ...Object.values(g.at)], g.W, g.H, { seed: 11, spread: tall ? 110 : 150 }), [g, tall]);

  useEffect(() => {
    const narrow = matchMedia("(max-width: 720px)");
    const fit = () => setTall(narrow.matches);
    fit();
    narrow.addEventListener("change", fit);
    void import("@marmotdata/icons/define.js");

    let p: Player | undefined;
    let cancelled = false;
    // Under reduced motion the player runs straight to how the story ends, and the captions show
    // either sheet's ending as a still.
    void import("@site/src/marketing/demo").then(({ play }) => {
      if (cancelled || !root.current) return;
      let cur = { ...START };
      const set = (patch: Partial<State>) => setS((cur = { ...cur, ...patch }));
      const along = (path: SVGPathElement | null, t: number) => {
        if (!path) return null;
        const at = path.getPointAtLength(path.getTotalLength() * t);
        return { x: at.x, y: at.y };
      };
      p = play(root.current, {
        reset: () => set({ ...START }),
        async run(demo) {
          await demo.wait(T.start);
          // Unmapped: the agent tries one look-alike after another, then the customer table.
          set({ leg: "wander" });
          await demo.animate(T.wander, (t) => set({ dot: along(wanderRef.current, t), visited: Math.min(WANDER.length, Math.floor(t * WANDER.length + 0.05)) }));
          await demo.wait(T.lost);
          // Surveyed: one route to the certified table, then a query refused at the zone.
          set({ leg: null, dot: null });
          await demo.wait(T.pause);
          set({ leg: "route" });
          await demo.animate(T.route, (t) => set({ dot: along(routeRef.current, t) }));
          set({ arrived: true, dot: null });
          await demo.wait(T.arrived);
          set({ leg: "spur" });
          await demo.animate(T.spur, (t) => set({ dot: along(spurRef.current, t) }));
          set({ refused: true, dot: null });
          await demo.wait(T.end);
        },
        rest: 1200,
      });
      setPlayer(p);
    });
    return () => {
      cancelled = true;
      narrow.removeEventListener("change", fit);
      p?.destroy();
    };
  }, []);

  // On a phone the sheets are stacked, so someone who scrolls on to the second one is shown it
  // rather than left waiting for the first to finish above them.
  useEffect(() => {
    const sheet = withSheet.current;
    if (!tall || !player || !sheet) return;
    let showing = false;
    const skip = () => showing && player.elapsed < BOUNDS[1]! && player.seek(BOUNDS[1]!);
    const io = new IntersectionObserver(
      ([entry]) => {
        showing = !!entry?.isIntersecting;
        skip();
      },
      { threshold: 0.5 },
    );
    io.observe(sheet);
    // When the story starts over, it skips the first sheet again for as long as the second is showing.
    const timer = setInterval(skip, 250);
    return () => {
      io.disconnect();
      clearInterval(timer);
    };
  }, [tall, player]);

  const reached = (id: AssetId) => WANDER.indexOf(id) > -1 && WANDER.indexOf(id) < s.visited;
  const contours = (
    <g className="mkt-survey__ground">
      {ground.map((c, i) => (
        <path key={i} d={c.d} className={c.index ? "is-index" : undefined} />
      ))}
    </g>
  );

  return (
    <div className={`mkt-survey${tall ? " is-tall" : ""}`} ref={root}>
      <Beats points={HALVES} player={player} bounds={BOUNDS} className="mkt-chapter__points mkt-survey__halves" />

      <div className="mkt-survey__sheet is-without">
        <div className="mkt-survey__map">
          <svg viewBox={`0 0 ${g.W} ${g.H}`} className="mkt-survey__svg" aria-hidden="true">
            <path ref={wanderRef} d={wander} className="mkt-survey__wander" />
            {s.leg === "wander" && s.dot && <circle cx={s.dot.x} cy={s.dot.y} r={6} className="mkt-survey__dot" />}
          </svg>
          <Agent g={g}>
            <span className="mkt-survey__badge mkt-survey__badge--warn">
              <Key aria-hidden="true" />
              Read-only database password
            </span>
          </Agent>
          {ASSETS.map((a) => (
            <Asset key={a.id} g={g} {...a} state={reached(a.id) ? (a.id === "customers" ? "is-refused" : "is-traced") : ""}>
              {reached(a.id) && (
                <span className={`mkt-maphero__note ${a.id === "customers" ? "mkt-maphero__note--refused" : "mkt-maphero__note--warn"}`}>
                  {a.id === "customers" ? <Warning aria-hidden="true" /> : <Help aria-hidden="true" />}
                  {a.id === "customers" ? "Emails read" : "Is this revenue?"}
                </span>
              )}
            </Asset>
          ))}
        </div>
      </div>

      <div className="mkt-survey__sheet is-with" ref={withSheet}>
        <div className="mkt-survey__map">
          <svg viewBox={`0 0 ${g.W} ${g.H}`} className="mkt-survey__svg" aria-hidden="true">
            <defs>
              <pattern id="mkt-restricted" width="8" height="8" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
                <line x1="0" y1="0" x2="0" y2="8" className="mkt-survey__hatch" />
              </pattern>
            </defs>
            {contours}
            <g className="mkt-survey__zone">
              <circle cx={g.zone.x} cy={g.zone.y} r={g.zone.r} fill="url(#mkt-restricted)" />
              <circle cx={g.zone.x} cy={g.zone.y} r={g.zone.r} className="mkt-survey__fence" />
            </g>
            <path ref={routeRef} d={g.route} pathLength={1} className={`mkt-survey__route${s.leg === "route" || s.arrived ? " is-drawn" : ""}`} />
            <path ref={spurRef} d={g.spur} className={`mkt-survey__spur${s.leg === "spur" || s.refused ? " is-on" : ""}`} />
            {s.refused && <circle cx={g.stop.x} cy={g.stop.y} r={5} className="mkt-survey__stop" />}
            {(s.leg === "route" || s.leg === "spur") && s.dot && <circle cx={s.dot.x} cy={s.dot.y} r={6} className="mkt-survey__dot" />}
          </svg>
          <Agent g={g}>
            <span className="mkt-survey__badge">Over MCP</span>
          </Agent>
          {ASSETS.map((a) => (
            <Asset
              key={a.id}
              g={g}
              {...a}
              state={a.id === "daily" ? (s.arrived ? "is-found" : "") : a.id === "customers" ? (s.refused ? "is-refused" : "") : "is-quiet"}
            >
              {a.id === "daily" && s.arrived && (
                <span className="mkt-maphero__note">
                  <Check aria-hidden="true" />
                  Certified · 7 rows
                </span>
              )}
              {a.id === "customers" && s.refused && (
                <span className="mkt-maphero__note mkt-maphero__note--refused">
                  <Block aria-hidden="true" />
                  Refused
                </span>
              )}
            </Asset>
          ))}
          <span className="mkt-survey__zone-label" style={pct(g, g.zoneLabel)}>
            Personal data · needs a grant
          </span>
        </div>
      </div>

      {/* The sheets are drawn for the eye; this says what they show, for a screen reader. */}
      <p className="mm-visually-hidden">
        Two maps of the same tables. Without Marmot, the agent guesses between look-alike order tables with a database password, and ends up reading
        customer emails. With Marmot, it takes one logged route to the certified orders_daily table, and its query on the customer table is refused.
      </p>
    </div>
  );
}
