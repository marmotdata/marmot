import React, { useEffect } from "react";
import Layout from "@theme/Layout";
import Hero from "../components/Hero";
import BenefitsShowcase from "../components/BenefitsShowcase";
import QuickDeploy from "../components/QuickDeploy";
import ArchitectureComparison from "../components/ArchitectureComparison";
import DataSources from "../components/DataSources";
import PerformanceProof from "../components/PerformanceProof";
import MCPShowcase from "../components/MCPShowcase";
import CTA from "../components/CTA";

export default function Home(): JSX.Element {
  useEffect(() => {
    const logo = document.querySelector(".navbar__logo") as HTMLElement;
    if (logo) {
      logo.style.display = "none";
    }

    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            entry.target.classList.add("animate-in");
          }
        });
      },
      { threshold: 0.08, rootMargin: "0px 0px -40px 0px" },
    );

    document.querySelectorAll("[data-animate]").forEach((el) => {
      observer.observe(el);
    });

    return () => {
      const logo = document.querySelector(".navbar__logo") as HTMLElement;
      if (logo) {
        logo.style.display = "";
      }
      observer.disconnect();
    };
  }, []);

  return (
    <Layout
      title="The Open Source Context Layer for AI"
      description="The open-source context layer for AI. Catalog your tables, topics, queues, and APIs — then expose real metadata to AI agents through MCP."
    >
      <div className="bg-earthy-brown-50 dark:bg-gray-900 min-h-screen overflow-hidden">
        <Hero />
        <BenefitsShowcase />
        <MCPShowcase />
        <ArchitectureComparison />
        <QuickDeploy />
        <DataSources />
        <PerformanceProof />
        <CTA />
      </div>
    </Layout>
  );
}
