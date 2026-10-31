import React from "react";
import LegalPage from "@site/src/components/legal/LegalPage";
import PrivacyPolicy from "@site/src/components/legal/PrivacyPolicy";

export default function Privacy(): JSX.Element {
  return (
    <LegalPage
      title="Privacy policy"
      description="Marmot privacy policy: how we collect, use, and protect your personal data on marmotdata.io and Marmot Cloud."
      caption="Last updated 12 October 2026"
    >
      <PrivacyPolicy />
    </LegalPage>
  );
}
