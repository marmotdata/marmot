import React, { useEffect } from "react";
import Layout from "@theme/Layout";
import Hero from "../components/Hero";
import Stats from "../components/Stats";
import Benefits from "../components/BenefitsShowcase";
import DataSources from "../components/DataSources";
import Integrations from "../components/Integrations";
import CTA from "../components/CTA";

export default function Home(): JSX.Element {
  useEffect(() => {
    const logo = document.querySelector(".navbar__logo") as HTMLElement;
    if (logo) {
      logo.style.display = "none";
    }
    return () => {
      const logo = document.querySelector(".navbar__logo") as HTMLElement;
      if (logo) {
        logo.style.display = "";
      }
    };
  }, []);

  return (
    <Layout
      title={`Discover any data asset across your entire org in seconds`}
      description="Discover any data asset across your entire org in seconds"
    >
      <div className="bg-earthy-brown-50 min-h-screen">
        <Hero />
        <Stats />
        <Benefits />
        <DataSources />
        <Integrations />
        <CTA />
      </div>
    </Layout>
  );
}
