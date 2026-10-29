import React, { useEffect, useRef, useState } from "react";
import type { TemplateResult } from "lit";
import type { Player } from "@site/src/marketing/demo";
import type { DemoConfig } from "@site/src/marketing/demos";

// The size a screen is laid out at, as the app would be in a browser window.
const W = 1280;
const H = 760;
// Scrollbars belong to someone using the app, not to a picture of it. Most scroll areas are inside
// the elements' shadow roots, which page CSS cannot reach, so each root gets this sheet directly.
let hideScrollbars: CSSStyleSheet | undefined;
function quietScrollbars(root: Element | ShadowRoot, seen: WeakSet<ShadowRoot>) {
  if (typeof CSSStyleSheet === "undefined") return;
  hideScrollbars ??= (() => {
    const sheet = new CSSStyleSheet();
    sheet.replaceSync("*{scrollbar-width:none}*::-webkit-scrollbar{display:none}");
    return sheet;
  })();
  for (const el of root.querySelectorAll("*")) {
    const shadow = el.shadowRoot;
    if (!shadow) continue;
    if (!seen.has(shadow)) {
      seen.add(shadow);
      shadow.adoptedStyleSheets = [...shadow.adoptedStyleSheets, hideScrollbars];
    }
    quietScrollbars(shadow, seen);
  }
}

const reduced = () => typeof matchMedia !== "undefined" && matchMedia("(prefers-reduced-motion: reduce)").matches;

/**
 * A product screen at work: the real design-system components drawn by Lit at app size, scaled
 * to the column, with a camera that moves to what the script points at. The screen is inert, a
 * picture rather than a second app to tab through, and `label` tells assistive technology what
 * it shows. It plays only on screen and can be paused.
 */
export default function ScreenDemo<S extends { focus?: string }>(props: DemoConfig<S>): JSX.Element {
  const { label, screen, initial, steps, rest } = props;
  const root = useRef<HTMLElement>(null);
  const box = useRef<HTMLDivElement>(null);
  const stage = useRef<HTMLDivElement>(null);
  const [player, setPlayer] = useState<Player>();
  const [boxWidth, setBoxWidth] = useState(0);
  const [focus, setFocus] = useState<string | undefined>(initial.focus);
  const [camera, setCamera] = useState({ k: 1, x: 0, y: 0 });

  // The state lives outside React: the script changes it many times a second and only the Lit
  // screen and the camera need to hear about it.
  const state = useRef<S>({ ...initial });

  useEffect(() => {
    // Deferred a frame: setting state inside the callback re-lays out the page within the same
    // observation, which the browser reports as a ResizeObserver loop.
    let frame = 0;
    const observer = new ResizeObserver(([entry]) => {
      const width = entry?.contentRect.width ?? 0;
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => setBoxWidth((w) => (Math.abs(w - width) < 0.5 ? w : width)));
    });
    if (box.current) observer.observe(box.current);
    let p: Player | undefined;
    let cancelled = false;
    void Promise.all([import("@site/src/marketing/screens/elements"), import("lit"), import("@site/src/marketing/demo")]).then(
      async ([{ defineScreenElements }, { render }, { play }]) => {
        await defineScreenElements();
        if (cancelled || !root.current || !stage.current) return;
        const quieted = new WeakSet<ShadowRoot>();
        const draw = () => {
          if (stage.current) {
            render(screen(state.current) as TemplateResult, stage.current);
            // Elements render their shadow roots a moment after they are created.
            const el = stage.current;
            requestAnimationFrame(() => quietScrollbars(el, quieted));
          }
          setFocus(state.current.focus);
        };
        draw();
        p = play(root.current, {
          reset() {
            state.current = { ...initial };
            draw();
          },
          run: (demo) =>
            steps(demo, (patch) => {
              state.current = { ...state.current, ...patch };
              draw();
            }),
          rest,
        });
        setPlayer(p);
      },
    );
    return () => {
      cancelled = true;
      cancelAnimationFrame(frame);
      observer.disconnect();
      p?.destroy();
    };
    // The demo's config is fixed for the life of the page.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Frames the focused element with a margin, measured on the unscaled screen.
  useEffect(() => {
    const el = stage.current;
    const w = boxWidth;
    if (!el || !w) return;
    let stale = false;
    const fit = w / W;
    const frame = async () => {
      const target = focus ? el.querySelector<HTMLElement>(focus) : null;
      const main = el.querySelector<HTMLElement & { mainElement?: HTMLElement | null }>("mm-app-shell")?.mainElement;
      if (!target) {
        main?.scrollTo({ top: 0 });
        setCamera({ k: fit, x: 0, y: 0 });
        return;
      }
      const unit = el.getBoundingClientRect().width / W;
      // Bring it into the app's own scroll area first, as a person scrolling would.
      if (main) {
        const m = main.getBoundingClientRect();
        const r = target.getBoundingClientRect();
        let delta = 0;
        if (r.bottom > m.bottom) delta = (r.bottom - m.bottom) / unit + 24;
        if (r.top - delta * unit < m.top) delta = (r.top - m.top) / unit - 24;
        if (Math.abs(delta) > 1) {
          main.scrollBy({ top: delta, behavior: reduced() ? "auto" : "smooth" });
          await new Promise((done) => setTimeout(done, reduced() ? 0 : 450));
        }
      }
      if (stale) return;
      const s = el.getBoundingClientRect();
      const r = target.getBoundingClientRect();
      const pad = 32;
      const rx = (r.left - s.left) / unit - pad;
      const ry = (r.top - s.top) / unit - pad;
      const rw = r.width / unit + pad * 2;
      const rh = r.height / unit + pad * 2;
      // Never past 1.25× the real size: closer than that, interface text turns clumsy.
      const k = Math.max(fit, Math.min(w / rw, (w * (H / W)) / rh, 1.25));
      const vw = w / k;
      const vh = (w * (H / W)) / k;
      // Centre it, but never show past the screen's edges.
      setCamera({
        k,
        x: Math.max(0, Math.min(W - vw, rx + rw / 2 - vw / 2)),
        y: Math.max(0, Math.min(H - vh, ry + rh / 2 - vh / 2)),
      });
    };
    const t = requestAnimationFrame(() => void frame());
    return () => {
      stale = true;
      cancelAnimationFrame(t);
    };
  }, [focus, boxWidth]);

  return (
    <figure
      className="mkt-showcase mkt-showcase--bare"
      aria-label={label}
      ref={root}
      // No visible controls, so resting a pointer on a demo holds it still.
      onPointerEnter={() => player && !player.paused && player.toggle()}
      onPointerLeave={() => player?.paused && player.toggle()}
    >
      <div className="mkt-screen" ref={box}>
        <div
          className="mkt-screen__stage"
          ref={stage}
          // React 19 passes inert through as a boolean attribute.
          inert
          aria-hidden="true"
          style={
            {
              inlineSize: W,
              blockSize: H,
              "--screen-h": `${H}px`,
              transform: `scale(${camera.k}) translate(${-camera.x}px, ${-camera.y}px)`,
            } as React.CSSProperties
          }
        />
      </div>
      <p className="mm-visually-hidden">{label} Sample data.</p>
    </figure>
  );
}
