import React from "react";
import LegalPage from "@site/src/components/legal/LegalPage";
import DpaV1 from "@site/src/components/legal/DpaV1";

export default function DpaArchiveV1(): JSX.Element {
  return (
    <LegalPage
      title="Data processing agreement"
      metaTitle="Data processing agreement v1.0 (archived)"
      description="Archived version 1.0 of the Marmot Data Data Processing Agreement."
      caption="Version 1.0, last modified 23 August 2026"
      archived={{ label: "data processing agreement", to: "/dpa" }}
    >
      <DpaV1 />
    </LegalPage>
  );
}
