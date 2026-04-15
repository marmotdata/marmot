import React from "react";
import {
  useVersions,
  useActiveDocContext,
  useDocsVersionCandidates,
} from "@docusaurus/plugin-content-docs/client";
import { useHistory } from "@docusaurus/router";

export default function SidebarVersionDropdown(): JSX.Element | null {
  const versions = useVersions();
  const activeDocContext = useActiveDocContext();
  const candidates = useDocsVersionCandidates();
  const history = useHistory();

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
    <div className="sidebar-version-dropdown">
      <select
        value={activeVersion.name}
        onChange={(e) => {
          const selected = versions.find((v) => v.name === e.target.value);
          if (selected) {
            const targetDoc = getVersionTargetDoc(selected);
            if (targetDoc) {
              history.push(targetDoc.path);
            }
          }
        }}
      >
        {versions.map((version) => (
          <option key={version.name} value={version.name}>
            {version.label}
          </option>
        ))}
      </select>
    </div>
  );
}
