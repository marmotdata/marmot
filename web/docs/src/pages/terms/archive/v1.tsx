import React from "react";
import LegalPage from "@site/src/components/legal/LegalPage";
import TermsV1 from "@site/src/components/legal/TermsV1";

export default function TermsArchiveV1(): JSX.Element {
  return (
    <LegalPage
      title="Terms of service"
      metaTitle="Terms of service v1.0 (archived)"
      description="Archived version 1.0 of the Marmot Data Terms of Service."
      caption="Version 1.0, last updated 23 August 2026"
      archived={{ label: "terms of service", to: "/terms" }}
    >
      <TermsV1 />
    </LegalPage>
  );
}
