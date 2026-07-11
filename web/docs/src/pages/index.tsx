import React, { useEffect } from "react";
import Layout from "@theme/Layout";
import Hero from "../components/Hero";
import ContextProblem from "../components/ContextProblem";
import BenefitsShowcase from "../components/BenefitsShowcase";
import QuickDeploy from "../components/QuickDeploy";
import ArchitectureComparison from "../components/ArchitectureComparison";
import DataSources from "../components/DataSources";
import PerformanceProof from "../components/PerformanceProof";
import MCPShowcase from "../components/MCPShowcase";
import SecurityTrust from "../components/SecurityTrust";
import CTA from "../components/CTA";

export default function Home(): JSX.Element {
  useEffect(() => {
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
      observer.disconnect();
    };
  }, []);

  return (
    <Layout
      title="The Open Source Context Layer for Agents and Humans"
      description="Marmot is the open source context layer for agents and humans. Catalog what your data is, what it means, and who owns it, then make it available to your team and to every AI assistant through one MCP server."
    >
      <div className="bg-earthy-brown-50 dark:bg-gray-900 min-h-screen overflow-hidden">
        <Hero />
        <ContextProblem />
        <BenefitsShowcase />
        <MCPShowcase />
        <ArchitectureComparison />
        <QuickDeploy />
        <DataSources />
        <PerformanceProof />
        <SecurityTrust />
        <CTA />
      </div>
    </Layout>
  );
}
