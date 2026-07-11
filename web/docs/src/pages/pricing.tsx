import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type FormEvent,
} from "react";
import Layout from "@theme/Layout";
import Head from "@docusaurus/Head";
import { Icon } from "@iconify/react";

const API_BASE = "https://api.marmotdata.io";
const TURNSTILE_SITE_KEY = "0x4AAAAAAC14j-gGk5wzDj2N";

const FAQ_ITEMS = [
  {
    q: "Is Marmot really free?",
    a: "Yes. The open source core is MIT licensed with no usage limits. Self-host it for free, forever.",
  },
  {
    q: "Self-hosted vs Marmot Cloud?",
    a: "Cloud handles hosting, upgrades and backups for you. If you're comfortable running containers, self-hosting works great.",
  },
  {
    q: "When will Marmot Cloud be available?",
    a: "Currently in early development. Join the waitlist and you'll be among the first to get access.",
  },
  {
    q: "What professional services do you offer?",
    a: "Anything from building a custom connector to helping you deploy Marmot across your organization. Scoped to what you actually need.",
  },
];

const faqJsonLd = {
  "@context": "https://schema.org",
  "@type": "FAQPage",
  mainEntity: FAQ_ITEMS.map(({ q, a }) => ({
    "@type": "Question",
    name: q,
    acceptedAnswer: { "@type": "Answer", text: a },
  })),
};

// Preload the Turnstile script so it's ready when needed
function useTurnstileScript() {
  useEffect(() => {
    if ((window as any).turnstile) return;
    if (document.querySelector('script[src*="challenges.cloudflare.com"]')) return;
    const script = document.createElement("script");
    script.src =
      "https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit";
    script.async = true;
    document.head.appendChild(script);
  }, []);
}

// Renders + immediately executes on mount. Only mount when needed.
function Turnstile({ onToken }: { onToken: (token: string) => void }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const widgetIdRef = useRef<string | undefined>(undefined);

  useEffect(() => {
    function renderAndExecute() {
      if (!containerRef.current || widgetIdRef.current !== undefined) return;
      widgetIdRef.current = (window as any).turnstile.render(
        containerRef.current,
        {
          sitekey: TURNSTILE_SITE_KEY,
          execution: "execute",
          callback: onToken,
          "refresh-expired": "auto",
        },
      );
      (window as any).turnstile.execute(containerRef.current);
    }

    if ((window as any).turnstile) {
      renderAndExecute();
    } else {
      const script = document.querySelector(
        'script[src*="challenges.cloudflare.com"]',
      );
      script?.addEventListener("load", renderAndExecute);
    }

    return () => {
      if (
        widgetIdRef.current !== undefined &&
        (window as any).turnstile
      ) {
        (window as any).turnstile.remove(widgetIdRef.current);
        widgetIdRef.current = undefined;
      }
    };
  }, [onToken]);

  return <div ref={containerRef} />;
}

function WaitlistForm() {
  const [email, setEmail] = useState("");
  const [marketing, setMarketing] = useState(false);
  const [status, setStatus] = useState<
    "idle" | "loading" | "success" | "error"
  >("idle");
  const [errorMsg, setErrorMsg] = useState("");
  const [challenged, setChallenged] = useState(false);
  const formDataRef = useRef({ email: "", marketing: false });

  useTurnstileScript();

  const handleToken = useCallback(async (token: string) => {
    const data = formDataRef.current;
    try {
      const res = await fetch(`${API_BASE}/waitlist`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          email: data.email,
          marketing_consent: data.marketing,
          "cf-turnstile-response": token,
        }),
      });
      const json = await res.json();
      if (!res.ok) {
        setErrorMsg(json.error || "Something went wrong");
        setStatus("error");
        setChallenged(false);
        return;
      }
      setStatus("success");
    } catch {
      setErrorMsg("Could not reach the server. Please try again.");
      setStatus("error");
      setChallenged(false);
    }
  }, []);

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setStatus("loading");
    setErrorMsg("");
    formDataRef.current = { email, marketing };
    setChallenged(false);
    // Force remount by toggling off then on in next tick
    setTimeout(() => setChallenged(true), 0);
  }

  if (status === "success") {
    return (
      <div className="pt-2">
        <p className="text-sm font-medium text-earthy-green-700 dark:text-earthy-green-400">
          You're on the list! We'll let you know when it's ready.
        </p>
      </div>
    );
  }

  return (
    <form onSubmit={handleSubmit} className="pt-2 space-y-3">
      <div className="flex gap-2">
        <input
          type="email"
          required
          autoFocus
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          placeholder="you@company.com"
          className="flex-1 min-w-0 px-3 py-2 text-sm rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800/60 text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500/40 focus:border-earthy-terracotta-500"
        />
        <button
          type="submit"
          disabled={status === "loading"}
          className="inline-flex items-center justify-center px-5 py-2 text-sm font-semibold rounded-lg border-none cursor-pointer text-white bg-earthy-terracotta-700 hover:bg-earthy-terracotta-800 disabled:opacity-60 transition-all duration-200 hover:-translate-y-0.5 whitespace-nowrap"
        >
          {status === "loading" ? "Joining…" : "Join Waitlist"}
        </button>
      </div>
      <label className="flex items-start gap-2 cursor-pointer">
        <input
          type="checkbox"
          checked={marketing}
          onChange={(e) => setMarketing(e.target.checked)}
          className="mt-0.5 rounded border-gray-300 dark:border-gray-600 text-earthy-terracotta-600 focus:ring-earthy-terracotta-500"
        />
        <span className="text-xs text-gray-500 dark:text-gray-400">
          Send me product updates and news
        </span>
      </label>
      {challenged && <Turnstile onToken={handleToken} />}
      {status === "error" && (
        <p className="text-xs text-red-600 dark:text-red-400">{errorMsg}</p>
      )}
      <p className="text-[11px] text-gray-400 dark:text-gray-500">
        By signing up you agree to our{" "}
        <a href="/privacy" className="underline hover:text-gray-600 dark:hover:text-gray-300">
          privacy policy
        </a>
        .
      </p>
    </form>
  );
}

function WaitlistDialog({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const dialogRef = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;
    if (open && !dialog.open) {
      dialog.showModal();
      dialog.querySelector<HTMLInputElement>('input[type="email"]')?.focus();
    }
    if (!open && dialog.open) dialog.close();
  }, [open]);

  return (
    <dialog
      ref={dialogRef}
      onClose={onClose}
      onClick={(e) => {
        // The dialog itself is only the click target when the backdrop is hit
        if (e.target === dialogRef.current) onClose();
      }}
      className="w-full max-w-md rounded-2xl p-0 overflow-visible bg-white dark:bg-gray-800 border border-earthy-terracotta-200 dark:border-earthy-terracotta-700/50 shadow-lg hub-pulse backdrop:bg-gray-900/50 backdrop:backdrop-blur-sm"
    >
      {/* Form only mounts while open, so state resets between visits */}
      {open && (
        <div className="relative p-7">
          <img
            src="/img/marmot.svg"
            alt=""
            className="absolute -top-14 right-8 w-16 h-16 pointer-events-none"
          />
          <div className="flex items-center gap-2.5 mb-2">
            <h3 className="text-lg font-bold text-gray-900 dark:text-white mb-0">
              Marmot Cloud
            </h3>
            <span className="inline-flex items-center gap-1.5 text-[11px] font-medium text-gray-400 dark:text-gray-500 ml-auto">
              <span className="w-1.5 h-1.5 rounded-full bg-earthy-yellow-500 animate-pulse" />
              Coming soon
            </span>
            <button
              type="button"
              onClick={onClose}
              aria-label="Close"
              className="p-1.5 -mr-1.5 rounded-lg bg-transparent border-none cursor-pointer text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-earthy-terracotta-500/40"
            >
              <svg
                className="w-4 h-4"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </div>
          <p className="text-sm text-gray-500 dark:text-gray-400 mb-2">
            Managed, metered hosting for your fleet. Join the waitlist for
            early access and onboarding.
          </p>
          <WaitlistForm />
        </div>
      )}
    </dialog>
  );
}

function CheckIcon() {
  return (
    <svg
      className="w-3.5 h-3.5 text-earthy-green-600 dark:text-earthy-green-400 flex-shrink-0 mt-0.5"
      fill="none"
      stroke="currentColor"
      viewBox="0 0 24 24"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2.5}
        d="M5 13l4 4L19 7"
      />
    </svg>
  );
}

function ArrowIcon() {
  return (
    <svg
      className="w-4 h-4 ml-2 transition-transform duration-200 group-hover:translate-x-0.5"
      fill="none"
      stroke="currentColor"
      viewBox="0 0 24 24"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M13 7l5 5m0 0l-5 5m5-5H6"
      />
    </svg>
  );
}

function FAQItem({ question, answer }: { question: string; answer: string }) {
  const [open, setOpen] = useState(false);

  return (
    <div
      className="border-b border-gray-200/80 dark:border-gray-700/40 last:border-b-0"
      style={{ background: "transparent" }}
    >
      <div
        role="button"
        tabIndex={0}
        onClick={() => setOpen(!open)}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            setOpen(!open);
          }
        }}
        className="flex items-center justify-between py-5 cursor-pointer group"
        style={{
          background: "transparent",
          border: "none",
          padding: "1.25rem 0",
        }}
      >
        <span className="text-sm font-semibold text-gray-900 dark:text-white pr-4 group-hover:text-earthy-terracotta-700 dark:group-hover:text-earthy-terracotta-400 transition-colors duration-200">
          {question}
        </span>
        <svg
          className={`w-4 h-4 text-gray-400 dark:text-gray-500 flex-shrink-0 transition-transform duration-200 ${open ? "rotate-180" : ""
            }`}
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M19 9l-7 7-7-7"
          />
        </svg>
      </div>
      <div
        className="overflow-hidden transition-all duration-200"
        style={{ maxHeight: open ? "240px" : "0", opacity: open ? 1 : 0 }}
      >
        <p className="pb-5 text-sm text-gray-500 dark:text-gray-400 leading-relaxed">
          {answer}
        </p>
      </div>
    </div>
  );
}

/* ============================================================
   Lookup-based pricing model
   ============================================================ */

const FREE_LIMIT = 500_000; // lookups/month included on every plan
const TEAM_FLOOR = 1000; // € platform floor on Team
const TIER1_RATE = 0.001; // € per lookup, 500K to 10M
const TIER2_RATE = 0.0006; // € per lookup, 10M+
const TIER2_START = 10_000_000;

interface Breakdown {
  freeBand: number;
  tier1Units: number;
  tier2Units: number;
  tier1Cost: number;
  tier2Cost: number;
  usage: number;
  floorApplies: boolean;
  total: number;
}

function computeBreakdown(lookups: number): Breakdown {
  const freeBand = Math.min(lookups, FREE_LIMIT);
  const tier1Units = Math.max(0, Math.min(lookups, TIER2_START) - FREE_LIMIT);
  const tier2Units = Math.max(0, lookups - TIER2_START);
  const tier1Cost = tier1Units * TIER1_RATE;
  const tier2Cost = tier2Units * TIER2_RATE;
  const usage = tier1Cost + tier2Cost;
  const floorApplies = lookups > FREE_LIMIT && usage < TEAM_FLOOR;
  const total = lookups <= FREE_LIMIT ? 0 : Math.max(TEAM_FLOOR, usage);
  return {
    freeBand,
    tier1Units,
    tier2Units,
    tier1Cost,
    tier2Cost,
    usage,
    floorApplies,
    total,
  };
}

function fmtLookups(n: number): string {
  if (n >= 1_000_000) {
    const m = n / 1_000_000;
    return `${m % 1 === 0 ? m.toFixed(0) : m.toFixed(1)}M`;
  }
  if (n >= 1_000) return `${Math.round(n / 1_000)}K`;
  return `${Math.round(n)}`;
}

function fmtEur(n: number): string {
  return `€${Math.round(n).toLocaleString("en-US")}`;
}

/* ---------- Estimator ---------- */

const ACTIVITY = [
  { key: "light", label: "Light", perDay: 50, hint: "occasional automation" },
  { key: "typical", label: "Typical", perDay: 200, hint: "agents in the loop" },
  { key: "heavy", label: "Heavy", perDay: 800, hint: "always on fleets" },
];

const AGENT_DAYS = 30; // agents run every day
const HUMAN_LOOKUPS_PER_DAY = 15; // per person browsing the catalog
const HUMAN_DAYS = 22; // working days

// Log slider for direct volume entry (50K → 50M lookups)
const DIRECT_MIN = 50_000;
const DIRECT_MAX = 50_000_000;
function sliderToLookups(s: number): number {
  const t = s / 1000;
  const raw = DIRECT_MIN * Math.pow(DIRECT_MAX / DIRECT_MIN, t);
  // snap to a readable step
  const step = raw >= 1_000_000 ? 100_000 : raw >= 100_000 ? 10_000 : 1_000;
  return Math.round(raw / step) * step;
}
function lookupsToSlider(n: number): number {
  const clamped = Math.min(Math.max(n, DIRECT_MIN), DIRECT_MAX);
  return (
    (Math.log(clamped / DIRECT_MIN) / Math.log(DIRECT_MAX / DIRECT_MIN)) * 1000
  );
}

function Slider({
  value,
  min,
  max,
  step,
  onChange,
  ariaLabel,
}: {
  value: number;
  min: number;
  max: number;
  step: number;
  onChange: (n: number) => void;
  ariaLabel: string;
}) {
  return (
    <input
      type="range"
      min={min}
      max={max}
      step={step}
      value={value}
      aria-label={ariaLabel}
      onChange={(e) => onChange(Number(e.target.value))}
      className="w-full h-2 rounded-full appearance-none cursor-pointer bg-gray-200 dark:bg-gray-700"
      style={{ accentColor: "#d25a30" }}
    />
  );
}

function Estimator({ onJoinWaitlist }: { onJoinWaitlist: () => void }) {
  const [mode, setMode] = useState<"fleet" | "volume">("fleet");

  // Fleet inputs default to a representative Team plan fleet (~1.5M lookups)
  const [agents, setAgents] = useState(250);
  const [perAgent, setPerAgent] = useState(200);
  const [people, setPeople] = useState(50);

  // Direct volume input (stored as slider position)
  const [volSlider, setVolSlider] = useState(() => lookupsToSlider(2_000_000));

  const fleetLookups = useMemo(
    () =>
      Math.round(
        agents * perAgent * AGENT_DAYS +
        people * HUMAN_LOOKUPS_PER_DAY * HUMAN_DAYS,
      ),
    [agents, perAgent, people],
  );

  const directLookups = useMemo(() => sliderToLookups(volSlider), [volSlider]);

  const lookups = mode === "fleet" ? fleetLookups : directLookups;
  const b = useMemo(() => computeBreakdown(lookups), [lookups]);

  const isFree = lookups <= FREE_LIMIT;
  const isEnterprise = lookups > 30_000_000;
  const planName = isFree ? "Free" : isEnterprise ? "Enterprise" : "Team";
  const effectivePer1k =
    lookups > FREE_LIMIT ? (b.total / (lookups / 1000)) : 0;

  return (
    <div className="glass-card rounded-2xl p-6 sm:p-8">
      <div className="grid grid-cols-1 lg:grid-cols-5 gap-8">
        {/* Inputs */}
        <div className="lg:col-span-3">
          {/* Mode toggle */}
          <div className="inline-flex p-1 rounded-xl bg-gray-100 dark:bg-gray-800/70 mb-7">
            {[
              { key: "fleet", label: "Estimate my usage" },
              { key: "volume", label: "I know my lookup count" },
            ].map((opt) => (
              <button
                key={opt.key}
                onClick={() => setMode(opt.key as "fleet" | "volume")}
                className={`px-4 py-1.5 text-xs sm:text-sm font-semibold rounded-lg transition-all duration-200 ${mode === opt.key
                  ? "bg-white dark:bg-gray-700 text-gray-900 dark:text-white shadow-sm"
                  : "text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200"
                  }`}
              >
                {opt.label}
              </button>
            ))}
          </div>

          {mode === "fleet" ? (
            <div className="space-y-7">
              {/* Agents */}
              <div>
                <div className="flex items-baseline justify-between mb-2">
                  <label className="text-sm font-semibold text-gray-700 dark:text-gray-300">
                    AI agents
                  </label>
                  <span className="text-sm font-bold text-gray-900 dark:text-white tabular-nums">
                    {agents.toLocaleString("en-US")}
                  </span>
                </div>
                <Slider
                  value={agents}
                  min={0}
                  max={500}
                  step={1}
                  onChange={setAgents}
                  ariaLabel="Number of AI agents"
                />
              </div>

              {/* Activity */}
              <div>
                <label className="block text-sm font-semibold text-gray-700 dark:text-gray-300 mb-2">
                  Activity per agent
                </label>
                <div className="grid grid-cols-3 gap-2">
                  {ACTIVITY.map((a) => (
                    <button
                      key={a.key}
                      onClick={() => setPerAgent(a.perDay)}
                      className={`rounded-xl border px-3 py-2.5 text-left transition-all duration-200 ${perAgent === a.perDay
                        ? "border-earthy-terracotta-400 dark:border-earthy-terracotta-500 bg-earthy-terracotta-50/70 dark:bg-earthy-terracotta-900/20"
                        : "border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600"
                        }`}
                    >
                      <span className="block text-sm font-bold text-gray-900 dark:text-white">
                        {a.label}
                      </span>
                      <span className="block text-[11px] text-gray-500 dark:text-gray-400">
                        ~{a.perDay} lookups/day
                      </span>
                    </button>
                  ))}
                </div>
              </div>

              {/* People */}
              <div>
                <div className="flex items-baseline justify-between mb-2">
                  <label className="text-sm font-semibold text-gray-700 dark:text-gray-300">
                    People using the catalog
                  </label>
                  <span className="text-sm font-bold text-gray-900 dark:text-white tabular-nums">
                    {people.toLocaleString("en-US")}
                  </span>
                </div>
                <Slider
                  value={people}
                  min={0}
                  max={500}
                  step={1}
                  onChange={setPeople}
                  ariaLabel="Number of people using the catalog"
                />
              </div>

              <p className="text-[11px] text-gray-400 dark:text-gray-500 leading-relaxed">
                Estimate assumes about {AGENT_DAYS} active days per agent and
                about {HUMAN_LOOKUPS_PER_DAY} lookups per person per workday. A
                lookup is one question Marmot answers, so searches, metadata
                fetches, lineage traversals and ownership lookups all count.
              </p>
            </div>
          ) : (
            <div className="space-y-7">
              <div>
                <div className="flex items-baseline justify-between mb-2">
                  <label className="text-sm font-semibold text-gray-700 dark:text-gray-300">
                    Lookups / month
                  </label>
                  <span className="text-sm font-bold text-gray-900 dark:text-white tabular-nums">
                    {directLookups.toLocaleString("en-US")}
                  </span>
                </div>
                <Slider
                  value={volSlider}
                  min={0}
                  max={1000}
                  step={1}
                  onChange={setVolSlider}
                  ariaLabel="Monthly lookups"
                />
                <div className="flex justify-between mt-2 text-[11px] text-gray-400 dark:text-gray-500">
                  <span>50K</span>
                  <span>500K</span>
                  <span>10M</span>
                  <span>50M</span>
                </div>
              </div>
            </div>
          )}
        </div>

        {/* Results */}
        <div className="lg:col-span-2">
          <div className="rounded-2xl bg-earthy-brown-50/70 dark:bg-gray-900/50 border border-gray-200/70 dark:border-gray-700/40 p-6 h-full flex flex-col">
            <p className="text-[11px] font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500 mb-1">
              Estimated monthly lookups
            </p>
            <p className="text-3xl font-extrabold text-gray-900 dark:text-white tracking-tight mb-4">
              {lookups.toLocaleString("en-US")}
            </p>

            <div className="flex items-center gap-2 mb-5">
              <span className="inline-flex items-center gap-1.5 rounded-full bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 px-2.5 py-1 text-[11px] font-bold text-earthy-terracotta-700 dark:text-earthy-terracotta-300">
                <Icon icon="mdi:check-decagram" className="w-3.5 h-3.5" />
                {planName} plan
              </span>
              {b.floorApplies && (
                <span className="text-[11px] text-gray-400 dark:text-gray-500">
                  platform floor applies
                </span>
              )}
            </div>

            {/* Breakdown */}
            <div className="space-y-2 text-sm border-t border-gray-200/70 dark:border-gray-700/40 pt-4">
              <div className="flex justify-between text-gray-500 dark:text-gray-400">
                <span>First {fmtLookups(FREE_LIMIT)}</span>
                <span className="text-earthy-green-600 dark:text-earthy-green-400 font-medium">
                  Free
                </span>
              </div>
              {b.tier1Units > 0 && (
                <div className="flex justify-between text-gray-500 dark:text-gray-400">
                  <span>
                    {fmtLookups(b.tier1Units)} × €{TIER1_RATE}
                  </span>
                  <span>{fmtEur(b.tier1Cost)}</span>
                </div>
              )}
              {b.tier2Units > 0 && (
                <div className="flex justify-between text-gray-500 dark:text-gray-400">
                  <span>
                    {fmtLookups(b.tier2Units)} × €{TIER2_RATE}
                  </span>
                  <span>{fmtEur(b.tier2Cost)}</span>
                </div>
              )}
              {b.floorApplies && (
                <div className="flex justify-between text-gray-500 dark:text-gray-400">
                  <span>€1K platform floor</span>
                  <span>{fmtEur(TEAM_FLOOR)}</span>
                </div>
              )}
            </div>

            <div className="mt-auto pt-5">
              <div className="flex items-baseline justify-between">
                <span className="text-sm font-semibold text-gray-700 dark:text-gray-300">
                  {isFree ? "You pay" : isEnterprise ? "From" : "Estimated total"}
                </span>
                <span className="text-3xl font-extrabold gradient-text tracking-tight">
                  {isFree ? "€0" : fmtEur(b.total)}
                  <span className="text-sm font-semibold text-gray-400 dark:text-gray-500">
                    /mo
                  </span>
                </span>
              </div>
              {!isFree && (
                <p className="text-[11px] text-gray-400 dark:text-gray-500 mt-1 text-right">
                  ≈ {fmtEur(effectivePer1k)} per 1K lookups
                </p>
              )}
              {isEnterprise && (
                <p className="text-[11px] text-earthy-terracotta-600 dark:text-earthy-terracotta-400 mt-2 text-right">
                  At this volume, Enterprise unlocks discounted rates.{" "}
                  <a href="mailto:support@marmotdata.io" className="underline">
                    Talk to sales
                  </a>
                  .
                </p>
              )}
              {isFree || isEnterprise ? (
                <a
                  href={isFree ? "/docs/introduction" : "mailto:support@marmotdata.io"}
                  className="group mt-4 w-full inline-flex items-center justify-center px-6 py-2.5 text-sm font-semibold rounded-xl text-white bg-earthy-terracotta-700 hover:bg-earthy-terracotta-800 shadow-sm hover:shadow-md transition-all duration-200 hover:-translate-y-0.5"
                >
                  {isFree ? "Start free" : "Talk to sales"}
                  <ArrowIcon />
                </a>
              ) : (
                <button
                  type="button"
                  onClick={onJoinWaitlist}
                  className="group mt-4 w-full inline-flex items-center justify-center px-6 py-2.5 text-sm font-semibold rounded-xl border-none cursor-pointer text-white bg-earthy-terracotta-700 hover:bg-earthy-terracotta-800 shadow-sm hover:shadow-md transition-all duration-200 hover:-translate-y-0.5"
                >
                  Join Cloud waitlist
                  <ArrowIcon />
                </button>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

/* ---------- Plans ---------- */

interface Plan {
  name: string;
  badge?: string;
  price: string;
  unit: string;
  tagline: string;
  features: string[];
  cta: { label: string; href: string };
  highlight?: boolean;
}

const plans: Plan[] = [
  {
    name: "Free",
    price: "€0",
    unit: "MIT, run it yourself · Cloud Free (capped)",
    tagline: "Open core. Run the OSS build yourself, or use Cloud Free.",
    features: [
      "Run it yourself: unlimited assets & lookups",
      "Cloud Free: up to 2,000 assets",
      "500K lookups / month included",
      "MCP server, REST API, SDK & CLI",
      "Search, lineage & glossary",
      "Community support",
    ],
    cta: { label: "Join Cloud waitlist", href: "#waitlist" },
  },
  {
    name: "Team",
    badge: "Most popular",
    price: "€1K",
    unit: "/mo floor + usage above 500K",
    tagline: "Managed cloud AI context that scales with your agents.",
    features: [
      "Everything in Free",
      "Managed Cloud or BYOC",
      "SAML / SSO & advanced RBAC",
      "Premium plugins & lineage",
      "€0.001/lookup 500K to 10M, €0.0006 above 10M",
      "Configurable limits per plan",
    ],
    cta: { label: "Join Cloud waitlist", href: "#waitlist" },
    highlight: true,
  },
  {
    name: "Enterprise",
    price: "from €6K",
    unit: "/mo + discounted usage",
    tagline: "For large agent fleets and regulated environments.",
    features: [
      "Everything in Team",
      "BYOC Transit Gateway / VPC",
      "Discounted marginal lookups",
      "Custom limits, SLAs & retention",
      "Dedicated support & onboarding",
      "Security & compliance review",
    ],
    cta: { label: "Talk to sales", href: "mailto:support@marmotdata.io" },
  },
];

export default function Pricing(): JSX.Element {
  const [waitlistOpen, setWaitlistOpen] = useState(false);

  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            entry.target.classList.add("animate-in");
          }
        });
      },
      { threshold: 0.08, rootMargin: "0px 0px -40px 0px" }
    );

    document.querySelectorAll("[data-animate]").forEach((el) => {
      observer.observe(el);
    });

    return () => observer.disconnect();
  }, []);

  return (
    <Layout
      title="Pricing"
      description="Price by lookups, not seats. Run Marmot yourself for free, or scale Marmot Cloud with your agents on metered pricing that grows with the value you capture."
    >
      <Head>
        <script type="application/ld+json">{JSON.stringify(faqJsonLd)}</script>
      </Head>
      <div className="bg-earthy-brown-50 dark:bg-gray-900 min-h-screen">
        {/* Hero */}
        <section className="pt-16 pb-10 px-4 sm:px-6 lg:px-8 gradient-mesh-hero">
          <div className="max-w-4xl mx-auto text-center">
            <p
              data-animate
              className="text-xs font-bold uppercase tracking-widest text-earthy-terracotta-600 dark:text-earthy-terracotta-400 mb-4"
            >
              Pricing
            </p>
            <h1
              data-animate
              data-animate-delay="1"
              className="text-4xl sm:text-5xl font-extrabold text-gray-900 dark:text-white mb-5 tracking-tight leading-[1.1]"
            >
              Price by lookups,{" "}
              <span className="gradient-text">not seats.</span>
            </h1>
            <p
              data-animate
              data-animate-delay="2"
              className="text-lg text-gray-500 dark:text-gray-400 leading-relaxed max-w-2xl mx-auto"
            >
              A lookup is one question Marmot answers about an asset. The more
              your agents work, the more value Marmot delivers, so usage scales
              with value, not with how many people you put behind a login.
            </p>
          </div>
        </section>

        {/* Estimator */}
        <section
          id="estimator"
          className="pb-14 px-4 sm:px-6 lg:px-8 scroll-mt-24"
        >
          <div data-animate className="max-w-5xl mx-auto">
            <Estimator onJoinWaitlist={() => setWaitlistOpen(true)} />
          </div>
        </section>

        <div className="section-divider max-w-2xl mx-auto" />

        {/* Plans */}
        <section className="py-14 px-4 sm:px-6 lg:px-8">
          <div className="max-w-6xl mx-auto">
            <div data-animate className="text-center mb-10">
              <h2 className="text-2xl sm:text-3xl font-bold text-gray-900 dark:text-white tracking-tight mb-3">
                One context layer, three ways to run it
              </h2>
              <p className="text-base text-gray-500 dark:text-gray-400 max-w-2xl mx-auto">
                Start free and run it yourself, scale on Cloud as your fleet
                grows, or bring your own cloud for enterprise.
              </p>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-6 items-stretch">
              {plans.map((plan, i) => (
                <div
                  key={plan.name}
                  data-animate
                  data-animate-delay={String(i + 1)}
                  className={`relative rounded-2xl p-7 flex flex-col ${plan.highlight
                    ? "bg-white dark:bg-gray-800 border-2 border-earthy-terracotta-400 dark:border-earthy-terracotta-500 shadow-lg md:-mt-3 md:mb-3"
                    : "glass-card"
                    }`}
                >
                  {plan.badge && (
                    <span className="absolute -top-3 left-1/2 -translate-x-1/2 inline-flex items-center rounded-full bg-earthy-terracotta-700 px-3 py-1 text-[11px] font-bold uppercase tracking-wide text-white shadow-sm">
                      {plan.badge}
                    </span>
                  )}
                  <h3 className="text-lg font-bold text-gray-900 dark:text-white mb-1">
                    {plan.name}
                  </h3>
                  <p className="text-sm text-gray-500 dark:text-gray-400 mb-5 min-h-[2.5rem]">
                    {plan.tagline}
                  </p>
                  <div className="mb-5">
                    <span className="text-3xl font-extrabold text-gray-900 dark:text-white tracking-tight">
                      {plan.price}
                    </span>
                    <p className="text-xs text-gray-400 dark:text-gray-500 mt-1">
                      {plan.unit}
                    </p>
                  </div>
                  {plan.cta.href === "#waitlist" ? (
                    <button
                      type="button"
                      onClick={() => setWaitlistOpen(true)}
                      className={`group inline-flex items-center justify-center px-5 py-2.5 text-sm font-semibold rounded-xl cursor-pointer transition-all duration-200 hover:-translate-y-0.5 mb-6 ${plan.highlight
                        ? "border-none text-white bg-earthy-terracotta-700 hover:bg-earthy-terracotta-800 shadow-sm hover:shadow-md"
                        : "text-gray-700 dark:text-gray-300 bg-white/70 dark:bg-gray-800/50 border border-solid border-gray-200 dark:border-gray-700 hover:border-earthy-terracotta-300 dark:hover:border-earthy-terracotta-600"
                        }`}
                    >
                      {plan.cta.label}
                      <ArrowIcon />
                    </button>
                  ) : (
                    <a
                      href={plan.cta.href}
                      className={`group inline-flex items-center justify-center px-5 py-2.5 text-sm font-semibold rounded-xl transition-all duration-200 hover:-translate-y-0.5 mb-6 ${plan.highlight
                        ? "text-white bg-earthy-terracotta-700 hover:bg-earthy-terracotta-800 shadow-sm hover:shadow-md"
                        : "text-gray-700 dark:text-gray-300 bg-white/70 dark:bg-gray-800/50 border border-gray-200 dark:border-gray-700 hover:border-earthy-terracotta-300 dark:hover:border-earthy-terracotta-600"
                        }`}
                    >
                      {plan.cta.label}
                      <ArrowIcon />
                    </a>
                  )}
                  <ul className="space-y-2.5">
                    {plan.features.map((f) => (
                      <li key={f} className="flex items-start gap-2">
                        <CheckIcon />
                        <span className="text-sm text-gray-600 dark:text-gray-400">
                          {f}
                        </span>
                      </li>
                    ))}
                  </ul>
                </div>
              ))}
            </div>
          </div>
        </section>

        <div className="section-divider max-w-2xl mx-auto" />

        {/* What's a lookup */}
        <section className="py-14 px-4 sm:px-6 lg:px-8">
          <div className="max-w-6xl mx-auto grid grid-cols-1 lg:grid-cols-2 gap-8 items-center">
            <div data-animate>
              <p className="text-xs font-bold uppercase tracking-widest text-earthy-terracotta-600 dark:text-earthy-terracotta-400 mb-3">
                Economics
              </p>
              <h2 className="text-2xl font-bold text-gray-900 dark:text-white tracking-tight mb-3">
                Usage scales with value
              </h2>
              <p className="text-sm text-gray-500 dark:text-gray-400 leading-relaxed mb-4">
                A lookup is one question Marmot answers about an asset: a
                search, a metadata fetch, a lineage traversal, an ownership
                resolution. Humans and agents both count, through MCP, the API,
                the SDK or the UI.
              </p>
              <p className="text-sm text-gray-500 dark:text-gray-400 leading-relaxed">
                Your first 500K lookups each month are free on every plan. Team
                adds a €1K platform floor and you only pay above it once usage
                grows, and higher plans get cheaper marginal lookups.
              </p>
            </div>

            <div
              data-animate
              data-animate-delay="1"
              className="hidden lg:flex items-center justify-center"
            >
              <img
                src="/img/marmot.svg"
                alt="Marmot"
                className="w-36 h-36 float-gentle"
              />
            </div>
          </div>
        </section>

        {/* FAQ */}
        <section className="pb-20 px-4 sm:px-6 lg:px-8">
          <div className="max-w-6xl mx-auto">
            <h2
              data-animate
              className="text-2xl font-bold text-gray-900 dark:text-white text-center mb-8 tracking-tight"
            >
              Frequently asked questions
            </h2>
            <div data-animate className="max-w-3xl mx-auto">
              {[
                {
                  q: "What exactly is a lookup?",
                  a: "A lookup is one question Marmot answers about an asset: a search, a metadata fetch, a lineage traversal or an ownership resolution. It doesn't matter whether the question comes from an AI agent through MCP, from the SDK or API, or from a person using the UI. Each answered question is one lookup.",
                },
                {
                  q: "How does the €1K Team floor work?",
                  a: "Team starts at a €1,000/month platform floor. Your first 500K lookups every month are free, and beyond that you pay per lookup, but your bill only rises above €1,000 once usage exceeds the floor (around 1.5M lookups). At 2M lookups you pay about €1.5K, at 6M about €5.5K.",
                },
                {
                  q: "Can I still run it myself for free?",
                  a: "Yes. Marmot is open core and MIT licensed. Run the OSS build yourself with unlimited assets and unlimited lookups, forever. The metered pricing only applies to managed Marmot Cloud and BYOC deployments.",
                },
                {
                  q: "Why price on lookups instead of seats?",
                  a: "Agents don't have seats. Pricing by the seat punishes you for giving more people and more agents access to context, the opposite of what a context layer should do. Lookups track the value Marmot actually delivers, so cost scales with usage, not headcount.",
                },
                {
                  q: "Can I cap my spend?",
                  a: "Yes. Every paid plan supports configurable limits per plan so you can set a ceiling on lookups and avoid surprise bills. Enterprise adds custom limits, SLAs and retention.",
                },
                {
                  q: "Does Marmot store our actual data?",
                  a: "No. Marmot is a context layer, so it stores metadata about your assets (schemas, ownership, descriptions, lineage and statistics), not the rows, messages or payloads inside them. Run it yourself and even that metadata stays inside your own VPC, under your own access controls. Lookups are also auditable and capped per plan.",
                },
                {
                  q: "When will Marmot Cloud be available?",
                  a: "Cloud is in active development. Join the waitlist and you'll be among the first to get access, with onboarding help to map your fleet to the right plan.",
                },
              ].map(({ q, a }) => (
                <FAQItem key={q} question={q} answer={a} />
              ))}
            </div>
          </div>
        </section>
      </div>
      <WaitlistDialog
        open={waitlistOpen}
        onClose={() => setWaitlistOpen(false)}
      />
    </Layout>
  );
}
