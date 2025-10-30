import React, { useEffect } from "react";
import Layout from "@theme/Layout";
import Hero from "../components/Hero";
import Value from "../components/Value";
import DataSources from "../components/DataSources";
import CTA from "../components/CTA";
import Benefits from "../components/BenefitsShowcase";
import Integrations from "../components/Integrations";
import {
  faSearch,
  faProjectDiagram,
  faChartLine,
} from "@fortawesome/free-solid-svg-icons";

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
      title={`Discover any data asset across your entire stack in seconds`}
      description="Discover any data asset across your entire stack in seconds"
    >
      <div className="bg-earthy-brown-50 min-h-screen">
        <Hero />
        <Value />
        <Benefits />
        <Integrations />
        <DataSources />
        <CTA />
      </div>
    </Layout>
  );
}
