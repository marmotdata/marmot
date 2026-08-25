import React, { useEffect } from "react";
import Layout from "@theme/Layout";
import Hero from "../components/Hero";
import ContextDiagram from "../components/ContextDiagram";
import MCPShowcase from "../components/MCPShowcase";
import ProductTour from "../components/ProductTour";
import QuickDeploy from "../components/QuickDeploy";
import DataSources from "../components/DataSources";
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
        <ContextDiagram />
        <MCPShowcase />
        <ProductTour />
        <QuickDeploy />
        <DataSources />
        <CTA />
      </div>
    </Layout>
  );
}
