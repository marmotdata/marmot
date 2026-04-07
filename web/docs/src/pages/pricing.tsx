import React, {
  useCallback,
  useEffect,
  useRef,
  useState,
  type FormEvent,
} from "react";
import Layout from "@theme/Layout";
import { Icon } from "@iconify/react";

const API_BASE = "https://api.marmotdata.io";
const TURNSTILE_SITE_KEY = "0x4AAAAAAC14j-gGk5wzDj2N";

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
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          placeholder="you@company.com"
          className="flex-1 min-w-0 px-3 py-2 text-sm rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800/60 text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500/40 focus:border-earthy-terracotta-500"
        />
        <button
          type="submit"
          disabled={status === "loading"}
          className="inline-flex items-center justify-center px-5 py-2 text-sm font-semibold rounded-lg text-white bg-earthy-terracotta-700 hover:bg-earthy-terracotta-800 disabled:opacity-60 transition-all duration-200 hover:-translate-y-0.5 whitespace-nowrap"
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

function ContactForm() {
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [message, setMessage] = useState("");
  const [status, setStatus] = useState<
    "idle" | "loading" | "success" | "error"
  >("idle");
  const [errorMsg, setErrorMsg] = useState("");
  const [challenged, setChallenged] = useState(false);
  const formDataRef = useRef({ name: "", email: "", message: "" });

  useTurnstileScript();

  const handleToken = useCallback(async (token: string) => {
    const data = formDataRef.current;
    try {
      const res = await fetch(`${API_BASE}/contact`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: data.name,
          email: data.email,
          message: data.message,
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
    formDataRef.current = { name, email, message };
    setChallenged(false);
    setTimeout(() => setChallenged(true), 0);
  }

  if (status === "success") {
    return (
      <div className="pt-2">
        <p className="text-sm font-medium text-earthy-green-700 dark:text-earthy-green-400">
          Thanks! I'll be in touch soon.
        </p>
      </div>
    );
  }

  return (
    <form onSubmit={handleSubmit} className="pt-2 space-y-2.5">
      <input
        type="text"
        required
        value={name}
        onChange={(e) => setName(e.target.value)}
        placeholder="Name"
        className="w-full px-3 py-2 text-sm rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800/60 text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500/40 focus:border-earthy-terracotta-500"
      />
      <input
        type="email"
        required
        value={email}
        onChange={(e) => setEmail(e.target.value)}
        placeholder="Email"
        className="w-full px-3 py-2 text-sm rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800/60 text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500/40 focus:border-earthy-terracotta-500"
      />
      <textarea
        required
        value={message}
        onChange={(e) => setMessage(e.target.value)}
        placeholder="Tell us about your project…"
        rows={3}
        className="w-full px-3 py-2 text-sm rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800/60 text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500/40 focus:border-earthy-terracotta-500 resize-none"
      />
      <button
        type="submit"
        disabled={status === "loading"}
        className="group w-full inline-flex items-center justify-center px-6 py-2.5 text-sm font-semibold rounded-xl text-gray-700 dark:text-gray-300 bg-white/70 dark:bg-gray-800/50 border border-gray-200 dark:border-gray-700 hover:border-earthy-terracotta-300 dark:hover:border-earthy-terracotta-600 disabled:opacity-60 transition-all duration-200 hover:-translate-y-0.5"
      >
        {status === "loading" ? "Sending…" : "Send"}
        {status !== "loading" && <ArrowIcon />}
      </button>
      {challenged && <Turnstile onToken={handleToken} />}
      {status === "error" && (
        <p className="text-xs text-red-600 dark:text-red-400">{errorMsg}</p>
      )}
      <p className="text-[11px] text-gray-400 dark:text-gray-500">
        By submitting you agree to our{" "}
        <a href="/privacy" className="underline hover:text-gray-600 dark:hover:text-gray-300">
          privacy policy
        </a>
        .
      </p>
    </form>
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
        style={{ maxHeight: open ? "200px" : "0", opacity: open ? 1 : 0 }}
      >
        <p className="pb-5 text-sm text-gray-500 dark:text-gray-400 leading-relaxed">
          {answer}
        </p>
      </div>
    </div>
  );
}

const highlights = [
  { text: "All plugins included", icon: "material-symbols:extension" },
  { text: "MCP for AI agents", icon: "material-symbols:smart-toy-outline" },
  { text: "REST API & CLI", icon: "material-symbols:terminal" },
  { text: "Search & lineage", icon: "material-symbols:account-tree" },
  { text: "Glossary & governance", icon: "material-symbols:shield-outline" },
  { text: "Deploy anywhere", icon: "material-symbols:cloud-upload" },
];

const cloudFeatures = [
  "Everything in the open-source edition",
  "No infrastructure to maintain",
  "Automatic upgrades and backups",
  "Built-in auth & free tier",
];

const servicesFeatures = [
  "Custom plugin / connector builds",
  "Integration into your data stack",
  "Architecture & deployment review",
  "Priority support & direct Slack",
];

export default function Pricing(): JSX.Element {
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
      description="Marmot is free and open source. Self-host for free or join the waitlist for Marmot Cloud."
    >
      <div className="bg-earthy-brown-50 dark:bg-gray-900 min-h-screen">
        {/* Open Source Hero — two-column */}
        <section className="pt-16 pb-12 px-4 sm:px-6 lg:px-8 gradient-mesh-hero">
          <div className="max-w-6xl mx-auto">
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-10 lg:gap-16 items-center">
              <div>
                <h1
                  data-animate
                  className="text-4xl sm:text-5xl font-extrabold text-gray-900 dark:text-white mb-4 tracking-tight leading-[1.1]"
                >
                  Marmot is{" "}
                  <span className="gradient-text">free and open source</span>
                </h1>
                <p
                  data-animate
                  data-animate-delay="1"
                  className="text-lg text-gray-500 dark:text-gray-400 leading-relaxed mb-7"
                >
                  Marmot is MIT licensed. Self-host it for free, no limits.
                </p>
                <div
                  data-animate
                  data-animate-delay="2"
                  className="flex flex-row items-center gap-3"
                >
                  <a
                    href="/docs/introduction"
                    className="group inline-flex items-center justify-center px-7 py-3 text-sm font-semibold rounded-xl text-white bg-earthy-terracotta-700 hover:bg-earthy-terracotta-800 shadow-sm hover:shadow-md transition-all duration-200 hover:-translate-y-0.5"
                  >
                    Get Started
                    <ArrowIcon />
                  </a>
                  <a
                    href="https://github.com/marmotdata/marmot"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="group inline-flex items-center justify-center px-7 py-3 text-sm font-semibold rounded-xl text-gray-700 dark:text-gray-300 bg-white/70 dark:bg-gray-800/50 border border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600 transition-all duration-200 hover:-translate-y-0.5"
                  >
                    <Icon icon="mdi:github" className="w-4 h-4 mr-2" />
                    GitHub
                  </a>
                </div>
              </div>

              <div
                data-animate
                data-animate-delay="3"
                className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-2 gap-3"
              >
                {highlights.map((h) => (
                  <div
                    key={h.text}
                    className="flex items-center gap-2.5 rounded-xl bg-white/60 dark:bg-gray-800/40 border border-gray-200/60 dark:border-gray-700/40 px-3.5 py-3"
                  >
                    <Icon
                      icon={h.icon}
                      className="w-5 h-5 text-earthy-terracotta-600 dark:text-earthy-terracotta-400 flex-shrink-0"
                    />
                    <span className="text-sm font-medium text-gray-700 dark:text-gray-300">
                      {h.text}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </section>

        <div className="section-divider max-w-2xl mx-auto" />

        {/* Cloud & Services — side by side */}
        <section className="py-12 px-4 sm:px-6 lg:px-8">
          <div className="max-w-6xl mx-auto">
            <div data-animate className="text-center mb-8">
              <h2 className="text-2xl sm:text-3xl font-bold text-gray-900 dark:text-white tracking-tight">
                Need more?
              </h2>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              {/* Marmot Cloud */}
              <div
                data-animate
                data-animate-delay="1"
                className="glass-card rounded-2xl p-7 flex flex-col"
              >
                <div className="flex items-center gap-2.5 mb-2">
                  <div className="w-8 h-8 rounded-lg bg-earthy-blue-100 dark:bg-earthy-blue-900/30 flex items-center justify-center">
                    <Icon
                      icon="material-symbols:cloud"
                      className="w-4.5 h-4.5 text-earthy-blue-700 dark:text-earthy-blue-400"
                    />
                  </div>
                  <h3 className="text-lg font-bold text-gray-900 dark:text-white">
                    Marmot Cloud
                  </h3>
                  <span className="inline-flex items-center gap-1.5 text-[11px] font-medium text-gray-400 dark:text-gray-500 ml-auto">
                    <span className="w-1.5 h-1.5 rounded-full bg-earthy-yellow-500 animate-pulse" />
                    Coming soon
                  </span>
                </div>
                <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">
                  Managed hosting so you can focus on your data. Free tier
                  included.
                </p>
                <ul className="space-y-2 mb-6">
                  {cloudFeatures.map((f) => (
                    <li key={f} className="flex items-start gap-2">
                      <CheckIcon />
                      <span className="text-sm text-gray-600 dark:text-gray-400">
                        {f}
                      </span>
                    </li>
                  ))}
                </ul>
                <WaitlistForm />
              </div>

              {/* Professional Services */}
              <div
                data-animate
                data-animate-delay="2"
                className="glass-card rounded-2xl p-7 flex flex-col"
              >
                <div className="flex items-center gap-2.5 mb-2">
                  <div className="w-8 h-8 rounded-lg bg-earthy-green-100 dark:bg-earthy-green-900/30 flex items-center justify-center">
                    <Icon
                      icon="material-symbols:handshake"
                      className="w-4.5 h-4.5 text-earthy-green-700 dark:text-earthy-green-400"
                    />
                  </div>
                  <h3 className="text-lg font-bold text-gray-900 dark:text-white">
                    Professional Services
                  </h3>
                </div>
                <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">
                  Hands-on help getting Marmot into your stack. Scoped per
                  engagement.
                </p>
                <ul className="space-y-2 mb-6">
                  {servicesFeatures.map((f) => (
                    <li key={f} className="flex items-start gap-2">
                      <CheckIcon />
                      <span className="text-sm text-gray-600 dark:text-gray-400">
                        {f}
                      </span>
                    </li>
                  ))}
                </ul>
                <ContactForm />
              </div>
            </div>
          </div>
        </section>

        {/* FAQ — 2x2 grid */}
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
                  q: "Is Marmot really free?",
                  a: "Yes. MIT licensed with no feature gating, no usage limits, and no telemetry. Self-host it for free, forever.",
                },
                {
                  q: "Self-hosted vs Marmot Cloud?",
                  a: "Same software. Cloud just handles hosting, upgrades, and backups. If you're comfortable running containers, self-hosting works great.",
                },
                {
                  q: "When will Marmot Cloud be available?",
                  a: "Currently in early development. Join the waitlist and you'll be among the first to get access.",
                },
                {
                  q: "What professional services do you offer?",
                  a: "Anything from building a custom connector to helping you deploy Marmot across your organization. Scoped to what you actually need.",
                },
              ].map(({ q, a }) => (
                <FAQItem key={q} question={q} answer={a} />
              ))}
            </div>
          </div>
        </section>
      </div>
    </Layout>
  );
}
