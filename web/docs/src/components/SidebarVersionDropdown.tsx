import React, { useId } from "react";
import {
  useVersions,
  useActiveDocContext,
  useDocsVersionCandidates,
} from "@docusaurus/plugin-content-docs/client";
import { useHistory } from "@docusaurus/router";
import ExpandMore from "~icons/material-symbols/expand-more";

/** The docs version picker at the top of the sidebar: a native select in the design system's style. */
export default function SidebarVersionDropdown(): JSX.Element | null {
  const versions = useVersions();
  const activeDocContext = useActiveDocContext();
  const candidates = useDocsVersionCandidates();
  const history = useHistory();
  const id = useId();

  if (versions.length <= 1) {
    return null;
  }

  const activeVersion = candidates[0] ?? versions[0];

  function getVersionTargetDoc(version: (typeof versions)[0]) {
    return (
      activeDocContext.alternateDocVersions[version.name] ??
      version.docs.find((doc) => doc.id === version.mainDocId)
    );
  }

  return (
    <div className="sidebar-version">
      <label className="sidebar-version__label" htmlFor={id}>
        Version
      </label>
      <div className="sidebar-version__control">
        <select
          id={id}
          className="sidebar-version__select"
          value={activeVersion.name}
          onChange={(e) => {
            const version = versions.find((v) => v.name === e.target.value);
            const targetDoc = version && getVersionTargetDoc(version);
            if (targetDoc) history.push(targetDoc.path);
          }}
        >
          {versions.map((version) => (
            <option key={version.name} value={version.name}>
              {version.label}
            </option>
          ))}
        </select>
        <ExpandMore aria-hidden="true" className="sidebar-version__caret" />
      </div>
    </div>
  );
}
