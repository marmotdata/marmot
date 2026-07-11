import React from "react";
import { Icon } from "@iconify/react";

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
  { icon: string; cls: string; label: string }
> = {
  yes: {
    icon: "mdi:check-circle",
    cls: "text-green-600 dark:text-green-400",
    label: "Yes:",
  },
  no: {
    icon: "mdi:close-circle",
    cls: "text-gray-400 dark:text-gray-500",
    label: "No:",
  },
  partial: {
    icon: "mdi:circle-slice-4",
    cls: "text-amber-500 dark:text-amber-400",
    label: "Partial:",
  },
};

function normaliseCell(cell: ComparisonCell | string): ComparisonCell {
  return typeof cell === "string" ? { text: cell } : cell;
}

// Reusable, styled comparison matrix. Renders a semantic <table> with real text
// in every cell (status icons are supplementary, with a visually hidden label),
// so it stays readable for search crawlers and LLMs while looking better than a
// plain markdown table.
export function ComparisonTable({
  columns,
  rows,
  highlightColumn,
  caption,
}: ComparisonTableProps): JSX.Element {
  return (
    <div className="my-6 overflow-x-auto rounded-xl border border-gray-200 dark:border-gray-700">
      <table className="w-full border-collapse text-sm m-0">
        {caption && <caption className="sr-only">{caption}</caption>}
        <thead>
          <tr className="bg-gray-50 dark:bg-gray-800">
            <th scope="col" className="px-4 py-3" />
            {columns.map((col, i) => (
              <th
                key={col}
                scope="col"
                className={`text-left px-4 py-3 font-bold ${
                  i === highlightColumn
                    ? "text-[var(--ifm-color-primary)]"
                    : "text-gray-900 dark:text-white"
                }`}
              >
                {col}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, ri) => (
            <tr
              key={row.feature}
              className={
                ri % 2 ? "bg-gray-50/60 dark:bg-gray-800/40" : undefined
              }
            >
              <th
                scope="row"
                className="text-left align-top px-4 py-3 font-semibold text-gray-700 dark:text-gray-300"
              >
                {row.feature}
              </th>
              {row.cells.map((raw, ci) => {
                const cell = normaliseCell(raw);
                const meta = cell.status ? STATUS_META[cell.status] : null;
                return (
                  <td
                    key={ci}
                    className={`align-top px-4 py-3 text-gray-700 dark:text-gray-300 ${
                      ci === highlightColumn
                        ? "bg-[var(--ifm-color-primary)]/[0.05]"
                        : ""
                    }`}
                  >
                    <span className="flex items-start gap-2">
                      {meta && (
                        <>
                          <Icon
                            icon={meta.icon}
                            aria-hidden="true"
                            className={`mt-0.5 shrink-0 w-4 h-4 ${meta.cls}`}
                          />
                          <span className="sr-only">{meta.label}</span>
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
