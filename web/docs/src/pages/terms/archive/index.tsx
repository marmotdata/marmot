import React from "react";
import LegalPage, { VersionList } from "@site/src/components/legal/LegalPage";

export default function TermsArchive(): JSX.Element {
  return (
    <LegalPage
      title="Terms of service archive"
      description="Every version of the Marmot Data Terms of Service."
    >
      <VersionList
        versions={[
          { label: "Version 2.0", to: "/terms", detail: "Last updated 8 October 2026. Current." },
          {
            label: "Version 1.0",
            to: "/terms/archive/v1",
            detail: 'Last updated 23 August 2026. Archived. Recorded in Marmot Cloud as "2026-08-29".',
          },
        ]}
      />
    </LegalPage>
  );
}
