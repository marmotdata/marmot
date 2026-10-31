import React from "react";
import LegalPage, { VersionList } from "@site/src/components/legal/LegalPage";

export default function DpaArchive(): JSX.Element {
  return (
    <LegalPage
      title="Data processing agreement archive"
      description="Every version of the Marmot Data Data Processing Agreement."
    >
      <VersionList
        versions={[
          { label: "Version 1.1", to: "/dpa", detail: "Last modified 8 October 2026. Current." },
          { label: "Version 1.0", to: "/dpa/archive/v1", detail: "Last modified 23 August 2026. Archived." },
        ]}
      />
    </LegalPage>
  );
}
