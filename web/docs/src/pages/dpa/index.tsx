import React from "react";
import Link from "@docusaurus/Link";
import LegalPage from "@site/src/components/legal/LegalPage";
import DpaV1_1 from "@site/src/components/legal/DpaV1_1";

export default function Dpa(): JSX.Element {
  return (
    <LegalPage
      title="Data processing agreement"
      description="Marmot Data Data Processing Agreement between Marmot Data Ltd (Processor) and the Controller."
      caption="Version 1.1, last modified 8 October 2026"
    >
      <DpaV1_1 />
      <p>
        Previous versions of this DPA are available in the <Link to="/dpa/archive">archive</Link>.
      </p>
    </LegalPage>
  );
}
