import React from "react";
import DefaultNavbarItem from "@theme/NavbarItem/DefaultNavbarItem";
import {
  CONSOLE_URL,
  useCloudSignedIn,
} from "@site/src/hooks/useCloudSignedIn";

// The header's account actions: Sign in and Start free for visitors, a single Open console button for people already signed in to Marmot Cloud
export default function CloudAuthNavbarItem({ mobile }: { mobile?: boolean }) {
  const signedIn = useCloudSignedIn();

  if (signedIn) {
    return (
      <DefaultNavbarItem
        mobile={mobile}
        href={CONSOLE_URL}
        label="Open console"
        className="cloud-button"
      />
    );
  }

  return (
    <>
      <DefaultNavbarItem
        mobile={mobile}
        href="https://cloud.marmotdata.io/login"
        label="Sign in"
        className="signin-link"
      />
      <DefaultNavbarItem
        mobile={mobile}
        href="https://cloud.marmotdata.io/signup"
        label="Start free"
        className="cloud-button"
      />
    </>
  );
}
