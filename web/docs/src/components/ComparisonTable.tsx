import React from "react";
import clsx from "clsx";
import Check from "~icons/material-symbols/check";
import Close from "~icons/material-symbols/close";
import Partial from "~icons/material-symbols/remove";

export type CellStatus = "yes" | "no" | "partial";

export interface ComparisonCell {
  /** Optional tick / cross / partial marker shown before the text. */
  status?: CellStatus;
  /** Cell text. Always rendered, so the table stays scraper and LLM friendly. */
  text: string;
}

export interface ComparisonRow {
  /** Row label, rendered as a row header. */
  feature: string;
  /** One cell per column, in the same order as `columns`. */
  cells: (ComparisonCell | string)[];
}

interface ComparisonTableProps {
  columns: string[];
  rows: ComparisonRow[];
  /** Zero-based index of a column to visually emphasise, e.g. your own product. */
  highlightColumn?: number;
  /** Accessible summary, read by screen readers and useful context for scrapers. */
  caption?: string;
}

const STATUS_META: Record<
  CellStatus,
  { Icon: (props: React.SVGProps<SVGSVGElement>) => React.ReactNode; tone: string; label: string }
> = {
  yes: { Icon: Check, tone: "is-yes", label: "Yes:" },
  no: { Icon: Close, tone: "is-no", label: "No:" },
  partial: { Icon: Partial, tone: "is-partial", label: "Partial:" },
};

function normaliseCell(cell: ComparisonCell | string): ComparisonCell {
  return typeof cell === "string" ? { text: cell } : cell;
}

// A comparison matrix as a plain table with thin rules. Every cell carries its text; the status
// icon is supplementary, with a visually hidden word, so nothing relies on colour or shape alone.
export function ComparisonTable({
  columns,
  rows,
  highlightColumn,
  caption,
}: ComparisonTableProps): JSX.Element {
  return (
    <div className="doc-compare">
      <table>
        {caption && <caption className="mm-visually-hidden">{caption}</caption>}
        <thead>
          <tr>
            <td />
            {columns.map((col, i) => (
              <th key={col} scope="col" className={clsx(i === highlightColumn && "is-highlight")}>
                {col}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.feature}>
              <th scope="row">{row.feature}</th>
              {row.cells.map((raw, ci) => {
                const cell = normaliseCell(raw);
                const meta = cell.status ? STATUS_META[cell.status] : null;
                return (
                  <td key={ci} className={clsx(ci === highlightColumn && "is-highlight")}>
                    <span className="doc-compare__cell">
                      {meta && (
                        <>
                          <meta.Icon aria-hidden="true" className={clsx("doc-compare__mark", meta.tone)} />
                          <span className="mm-visually-hidden">{meta.label}</span>
                        </>
                      )}
                      <span>{cell.text}</span>
                    </span>
                  </td>
                );
              })}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
