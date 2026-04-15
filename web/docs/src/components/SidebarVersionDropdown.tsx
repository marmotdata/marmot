import React, { useState, useRef, useEffect } from "react";
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
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

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
    <div className="sidebar-version-dropdown" ref={ref}>
      <span className="sidebar-version-label">Version</span>
      <button
        className="sidebar-version-button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        aria-haspopup="listbox"
      >
        <span>{activeVersion.label}</span>
        <svg
          className={`sidebar-version-chevron${open ? " sidebar-version-chevron--open" : ""}`}
          width="12"
          height="12"
          viewBox="0 0 12 12"
          fill="none"
          aria-hidden="true"
        >
          <path
            d="M3 4.5L6 7.5L9 4.5"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
      </button>
      {open && (
        <ul className="sidebar-version-menu" role="listbox">
          {versions.map((version) => {
            const isActive = version.name === activeVersion.name;
            return (
              <li
                key={version.name}
                role="option"
                aria-selected={isActive}
                className={`sidebar-version-option${isActive ? " sidebar-version-option--active" : ""}`}
                onClick={() => {
                  const targetDoc = getVersionTargetDoc(version);
                  if (targetDoc) {
                    history.push(targetDoc.path);
                  }
                  setOpen(false);
                }}
              >
                {version.label}
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
