import React, { useEffect, useRef, useState } from "react";
import Link from "@docusaurus/Link";
import ScreenDemo from "@site/src/components/home/ScreenDemo";
import Beats from "@site/src/components/home/Beats";
import type { Player } from "@site/src/marketing/demo";
import {
  findDemo,
  queryDemo,
  traceDemo,
  understandDemo,
  type DemoConfig,
} from "@site/src/marketing/demos";
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
import Block from "~icons/material-symbols/block-outline";
import History from "~icons/material-symbols/history";

/*
 * The product tour, one tab per team. Each tab says what changes for that team and plays the
 * product doing it, with the points captioning the demo as it plays. Only the selected tab's demo
 * is mounted, so one screen plays at a time and a tab starts from the beginning when chosen.
 */

type IconType = React.ComponentType<{ "aria-hidden"?: string }>;
type Focus = { focus?: string; beat?: number };
interface Team {
  id: string;
  /** What the team gets, under its name on the tab. */
  caption: string;
  label: string;
  title: string;
  lede: string;
  /** One per beat of the demo, in order, so they can caption it. */
  points: { Icon: IconType; text: string }[];
  /** Where to read more: the docs for what the tab shows. */
  more: { to: string; label: string };
  demo: DemoConfig<Focus>;
}
// Each tab's demo drives a different screen; the list only has to hand them on.
const any = <S extends Focus>(d: DemoConfig<S>) =>
  d as unknown as DemoConfig<Focus>;

const teams: Team[] = [
  {
    id: "engineering",
    caption: "See what a change breaks",
    label: "Data engineering",
    title: "Engineers see what a change breaks before they merge it",
    lede: "Lineage follows data from source tables through every model to the dashboards that use it.",
    points: [
      { Icon: AccountTree, text: "Upstream and downstream, across tools" },
      { Icon: Sync, text: "From plugins and OpenLineage runs" },
      { Icon: Dashboard, text: "The dashboards a change would break" },
    ],
    more: { to: "/docs/open-lineage", label: "How lineage is collected" },
    demo: any(traceDemo),
  },
  {
    id: "governance",
    caption: "Decide what each agent can query",
    label: "Governance",
    title:
      "Governance decides what each agent can query, and sees every attempt",
    lede: "People and agents query through Marmot under the same rules. Each query is checked against the caller’s permissions before it reaches the database.",
    points: [
      {
        Icon: Terminal,
        text: "Read-only queries against the sources you connect",
      },
      { Icon: Block, text: "Refused before it reaches the database" },
      { Icon: History, text: "Who ran what, when and whether it was allowed" },
    ],
    more: {
      to: "/docs/Cloud/access-control",
      label: "How access control works",
    },
    demo: any(queryDemo),
  },
  {
    id: "analytics",
    caption: "Find the right table",
    label: "Analytics",
    title: "Analysts find the right table without asking in Slack",
    lede: "Search every connected source in plain words, from the browser, the CLI or an agent over MCP.",
    points: [
      {
        Icon: Search,
        text: "Tables, topics, dashboards and models in one list",
      },
      { Icon: FilterList, text: "Filter by type, owner, tag or any metadata" },
      { Icon: Extension, text: "Kept current by 70+ plugins" },
    ],
    more: { to: "/docs/Plugins", label: "See the plugins" },
    demo: any(findDemo),
  },
  {
    id: "finance",
    caption: "Report from certified numbers",
    label: "Finance",
    title: "Finance reports from the numbers your analysts certify",
    lede: "Owners, descriptions and glossary terms sit with every table, so people and agents answer from the same definitions.",
    points: [
      { Icon: Group, text: "Owners and teams on every asset" },
      { Icon: MenuBook, text: "Glossary terms that explain each field" },
      { Icon: Verified, text: "Certified fields marked as the ones to use" },
    ],
    more: { to: "/docs/glossary", label: "How the glossary works" },
    demo: any(understandDemo),
  },
];

function Panel({
  t,
  onDemo,
}: {
  t: Team;
  onDemo: (d: { player: Player; bounds: number[] }) => void;
}): JSX.Element {
  const [demo, setDemo] = useState<{ player: Player; bounds: number[] }>();
  return (
    <div className="mkt-chapter mkt-chapter--split">
      <div className="mkt-chapter__text">
        <div>
          <h3 className="mkt-chapter__title">{t.title}</h3>
          <p className="mkt-chapter__lede">{t.lede}</p>
        </div>
        <Beats points={t.points} player={demo?.player} bounds={demo?.bounds} />
        <Link className="mm-link mkt-chapter__more" to={t.more.to}>
          {t.more.label}
        </Link>
      </div>
      <ScreenDemo
        {...t.demo}
        onPlayer={(player, bounds) => {
          setDemo({ player, bounds });
          onDemo({ player, bounds });
        }}
      />
    </div>
  );
}

/**
 * The teams as large tabs across the width. The selected tab's line fills as its demo plays, and
 * when the demo ends the next team's starts, until someone chooses a tab themselves. Tabs follow
 * the WAI-ARIA tabs pattern: one tab stop, arrows, Home and End.
 */
export default function Teams(): JSX.Element {
  const [index, setIndex] = useState(0);
  const [auto, setAuto] = useState(true);
  const [demo, setDemo] = useState<{ player: Player; bounds: number[] }>();
  const tabs = useRef<(HTMLButtonElement | null)[]>([]);
  const strip = useRef<HTMLDivElement>(null);
  const cards = useRef<HTMLDivElement>(null);
  // A phone swipes between teams: every team is a card in one horizontal row, the tab names
  // follow the swipe, and nothing advances on its own.
  const [phone, setPhone] = useState(false);
  useEffect(() => {
    const q = window.matchMedia("(max-width: 720px)");
    const update = () => setPhone(q.matches);
    update();
    q.addEventListener("change", update);
    return () => q.removeEventListener("change", update);
  }, []);
  useEffect(() => {
    const row = cards.current;
    if (!phone || !row) return;
    let frame = 0;
    const onScroll = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => {
        const i = Math.round(row.scrollLeft / (row.scrollWidth / teams.length));
        setIndex((cur) =>
          cur === i ? cur : Math.max(0, Math.min(teams.length - 1, i)),
        );
      });
    };
    row.addEventListener("scroll", onScroll, { passive: true });
    return () => {
      cancelAnimationFrame(frame);
      row.removeEventListener("scroll", onScroll);
    };
  }, [phone]);

  // On a phone the tabs are one scrolling row; keep the selected one in view as the tour moves
  // on. Only the row scrolls, never the page, and on a desktop the row never overflows.
  useEffect(() => {
    const row = strip.current;
    const tab = tabs.current[index];
    if (!row || !tab || row.scrollWidth <= row.clientWidth) return;
    row.scrollTo({
      left: tab.offsetLeft - row.offsetLeft - 16,
      behavior: "smooth",
    });
  }, [index]);

  // Fills the selected tab's line from the demo's own clock, and moves on when it has played once.
  useEffect(() => {
    if (!demo) return;
    const end = demo.bounds[demo.bounds.length - 1] || 1;
    const tab = tabs.current[index];
    let frame = 0;
    const tick = () => {
      const p = demo.player.still ? 1 : Math.min(1, demo.player.elapsed / end);
      tab?.style.setProperty("--progress", String(p));
      if (p >= 1 && auto && !phone && !demo.player.still) {
        setDemo(undefined);
        setIndex((i) => (i + 1) % teams.length);
        return;
      }
      frame = requestAnimationFrame(tick);
    };
    frame = requestAnimationFrame(tick);
    return () => {
      cancelAnimationFrame(frame);
      tab?.style.setProperty("--progress", "0");
    };
  }, [demo, index, auto, phone]);

  const choose = (i: number, focus = false) => {
    setAuto(false);
    setDemo(undefined);
    setIndex(i);
    if (focus) tabs.current[i]?.focus();
    const card = cards.current?.children[i] as HTMLElement | undefined;
    if (phone && card)
      cards.current!.scrollTo({
        left: card.offsetLeft - cards.current!.offsetLeft,
        behavior: "smooth",
      });
  };
  const onKeyDown = (e: React.KeyboardEvent, i: number) => {
    const rtl = getComputedStyle(e.currentTarget).direction === "rtl";
    const next = { ArrowRight: rtl ? -1 : 1, ArrowLeft: rtl ? 1 : -1 }[e.key];
    if (next) choose((i + next + teams.length) % teams.length, true);
    else if (e.key === "Home") choose(0, true);
    else if (e.key === "End") choose(teams.length - 1, true);
    else return;
    e.preventDefault();
  };

  const t = teams[index]!;
  return (
    <div className="mkt-teams">
      <div
        className="mkt-teams__tabs"
        role="tablist"
        aria-label="Teams"
        ref={strip}
      >
        {teams.map((team, i) => (
          <button
            key={team.id}
            ref={(el) => {
              tabs.current[i] = el;
            }}
            type="button"
            role="tab"
            id={`team-tab-${team.id}`}
            aria-controls={`team-panel-${team.id}`}
            aria-selected={i === index}
            tabIndex={i === index ? 0 : -1}
            onClick={() => choose(i)}
            onKeyDown={(e) => onKeyDown(e, i)}
          >
            <span className="mkt-teams__line" aria-hidden="true" />
            <strong>{team.label}</strong>
            <span>{team.caption}</span>
          </button>
        ))}
      </div>
      {phone ? (
        <>
          <div className="mkt-teams__cards" ref={cards}>
            {teams.map((team) => (
              <div
                key={team.id}
                role="tabpanel"
                id={`team-panel-${team.id}`}
                aria-labelledby={`team-tab-${team.id}`}
              >
                <Panel t={team} onDemo={() => {}} />
              </div>
            ))}
          </div>
          <div className="mkt-teams__dots" aria-hidden="true">
            {teams.map((team, i) => (
              <span
                key={team.id}
                className={i === index ? "is-on" : undefined}
              />
            ))}
          </div>
        </>
      ) : (
        <div
          role="tabpanel"
          id={`team-panel-${t.id}`}
          aria-labelledby={`team-tab-${t.id}`}
        >
          <Panel key={t.id} t={t} onDemo={setDemo} />
        </div>
      )}
    </div>
  );
}
