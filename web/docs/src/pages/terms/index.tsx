import React from "react";
import Link from "@docusaurus/Link";
import LegalPage from "@site/src/components/legal/LegalPage";
import TermsV2 from "@site/src/components/legal/TermsV2";

export default function Terms(): JSX.Element {
  return (
    <LegalPage
      title="Terms of service"
      description="Marmot Data Terms of Service: the agreement between Marmot Data Ltd and organisations that use Marmot Cloud."
      caption="Version 2.0, last updated 8 October 2026"
    >
      <TermsV2 />
      <p>
        Previous versions of these Terms are available in the <Link to="/terms/archive">archive</Link>.
      </p>
    </LegalPage>
  );
}
