import React, { useEffect } from "react";
import Layout from "@theme/Layout";
import Hero from "../components/Hero";
import { benefits, BenefitRow } from "../components/BenefitsShowcase";
import QuickDeploy from "../components/QuickDeploy";
import ArchitectureComparison from "../components/ArchitectureComparison";
import DataSources from "../components/DataSources";
import Integrations from "../components/Integrations";
import PerformanceProof from "../components/PerformanceProof";
import CTA from "../components/CTA";

function BenefitSection({
  index,
}: {
  index: number;
}) {
  return (
    <section className="py-16 lg:py-24 px-4 sm:px-6 lg:px-8 bg-white dark:bg-gray-800">
      <div className="max-w-7xl mx-auto">
        <BenefitRow benefit={benefits[index]} reversed={index % 2 !== 0} />
      </div>
    </section>
  );
}

export default function Home(): JSX.Element {
  useEffect(() => {
    const logo = document.querySelector(".navbar__logo") as HTMLElement;
    if (logo) {
      logo.style.display = "none";
    }

    benefits.forEach((b) => {
      new Image().src = b.imageSrcLight;
      new Image().src = b.imageSrcDark;
    });

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
      title={`Discover any data asset across your entire org in seconds`}
      description="Discover any data asset across your entire org in seconds"
    >
      <div className="bg-earthy-brown-50 dark:bg-gray-900 min-h-screen overflow-hidden">
        <Hero />
        <BenefitSection index={0} />
        <BenefitSection index={1} />
        <QuickDeploy />
        <BenefitSection index={2} />
        <ArchitectureComparison />
        <BenefitSection index={3} />
        <DataSources />
        <PerformanceProof />
        <Integrations />
        <CTA />
      </div>
    </Layout>
  );
}
