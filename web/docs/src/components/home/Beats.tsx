import React, { useEffect, useRef, useState } from "react";
import type { Player } from "@site/src/marketing/demo";

type IconType = React.ComponentType<{ "aria-hidden"?: string }>;

/**
 * A chapter's points as captions for its demo: the one being shown is in ink with a hairline
 * filling along its rule for as long as it plays, and choosing a point plays the demo from there.
 * Under reduced motion, choosing one shows how that beat ends instead. Until the demo is ready, or
 * if its beats don't match the points, it is a plain list.
 */
export default function Beats({
  points,
  player,
  bounds,
  className = "mkt-chapter__points",
}: {
  /** Each point's main line, and an optional quiet line under it. */
  points: { Icon: IconType; text: string; detail?: string }[];
  player?: Player;
  /** Where each beat starts in playing time, then where the script ends. */
  bounds?: number[];
  className?: string;
}): JSX.Element {
  const live = !!player && bounds?.length === points.length + 1;
  const [active, setActive] = useState(0);
  const rows = useRef<(HTMLLIElement | null)[]>([]);

  useEffect(() => {
    if (!live || !player || !bounds) return;
    let frame = 0;
    const tick = () => {
      const t = player.elapsed;
      let i = points.length - 1;
      while (i > 0 && t < bounds[i]!) i--;
      const progress = Math.min(1, Math.max(0, (t - bounds[i]!) / (bounds[i + 1]! - bounds[i]! || 1)));
      rows.current.forEach((row, j) => row?.style.setProperty("--beat-progress", String(j === i ? progress : 0)));
      setActive(i);
      frame = requestAnimationFrame(tick);
    };
    frame = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(frame);
  }, [live, player, bounds, points.length]);

  return (
    <ul className={`${className} mkt-beats`} data-live={live || undefined}>
      {points.map(({ Icon, text, detail }, i) => (
        <li key={text} ref={(el) => void (rows.current[i] = el)} className={live && i === active ? "is-active" : undefined}>
          {live ? (
            <button type="button" aria-current={i === active ? "step" : undefined} onClick={() => player!.seek(player!.still ? bounds![i + 1]! - 1 : bounds![i]!)}>
              <Icon aria-hidden="true" />
              <Words text={text} detail={detail} />
            </button>
          ) : (
            <>
              <Icon aria-hidden="true" />
              <Words text={text} detail={detail} />
            </>
          )}
        </li>
      ))}
    </ul>
  );
}

function Words({ text, detail }: { text: string; detail?: string }): JSX.Element {
  if (!detail) return <>{text}</>;
  return (
    <span className="mkt-beats__words">
      <strong>{text}</strong>
      <span>{detail}</span>
    </span>
  );
}
