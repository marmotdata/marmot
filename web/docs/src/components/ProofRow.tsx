import React from "react";
import GoogleForStartups from "./GoogleForStartups";

const INTEGRATIONS = "70+";

function Fact({
  value,
  label,
}: {
  value: string;
  label: string;
}): JSX.Element {
  return (
    <span className="proof-fact">
      <span className="proof-value">{value}</span>
      <span className="proof-label">{label}</span>
    </span>
  );
}

export default function ProofRow(): JSX.Element {
  return (
    <div data-animate data-animate-delay="4" className="proof-row">
      <GoogleForStartups />
      <span className="proof-sep" aria-hidden="true" />
      <a
        className="proof-fact-link"
        href="https://plugins.marmotdata.io"
        target="_blank"
        rel="noopener noreferrer"
      >
        <Fact value={INTEGRATIONS} label="integrations" />
      </a>
      <span className="proof-sep" aria-hidden="true" />
      <a className="proof-fact-link" href="/pricing">
        <span className="proof-claim">Self-host or Cloud</span>
      </a>
    </div>
  );
}
