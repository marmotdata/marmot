import React from "react";
import Check from "~icons/material-symbols/check";
import Remove from "~icons/material-symbols/remove";
import { PLAN_LIST } from "@site/src/marketing/plans";
import { COMPARE, type Cell } from "./plans";

function Value({ cell }: { cell: Cell }): JSX.Element {
  if (cell === true)
    return (
      <span className="mkt-plans__yes">
        <Check aria-hidden="true" />
        <span className="mm-visually-hidden">Included</span>
      </span>
    );
  if (cell === false)
    return (
      <span className="mkt-plans__no">
        <Remove aria-hidden="true" />
        <span className="mm-visually-hidden">Not included</span>
      </span>
    );
  return <>{cell}</>;
}

/**
 * Every difference between the plans, grouped into capacity, features and support, with a
 * heading row per group. Ticks and dashes carry hidden words for screen readers. On a narrow
 * screen the table scrolls sideways with its row labels pinned, so the keyboard can reach it.
 */
export default function PlanCompare(): JSX.Element {
  return (
    <div className="mkt-plans" role="region" aria-label="Plans compared" tabIndex={0}>
      <table>
        <colgroup>
          <col />
          {PLAN_LIST.map((p) => (
            <col key={p.id} />
          ))}
        </colgroup>
        <thead>
          <tr>
            <td />
            {PLAN_LIST.map((p) => (
              <th scope="col" key={p.id}>
                {p.name}
              </th>
            ))}
          </tr>
        </thead>
        {COMPARE.map((group) => (
          <tbody key={group.title}>
            <tr className="mkt-plans__group">
              <th scope="colgroup" colSpan={PLAN_LIST.length + 1}>
                {group.title}
              </th>
            </tr>
            {group.rows.map((row) => (
              <tr key={row.label}>
                <th scope="row">{row.label}</th>
                {PLAN_LIST.map((p) => (
                  <td key={p.id}>
                    <Value cell={row.cells[p.id]} />
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        ))}
      </table>
    </div>
  );
}
