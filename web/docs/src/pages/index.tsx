import React, { useEffect } from "react";
import Layout from "@theme/Layout";
import Hero from "../components/Hero";
import Query from "../components/Query";
import Showcase from "../components/Showcase";
import Lineage from "../components/Lineage";
import Integrations from "../components/Integrations";
import {
  faSearch,
  faProjectDiagram,
  faCheckCircle,
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
      title={`Modern Data Discovery for Modern Teams`}
      description="Modern Data Discovery for Modern Teams"
    >
      <div className="bg-earthy-brown-50 min-h-screen">
        <Hero />

        <Showcase
          options={[
            {
              id: "discovery",
              label: "Discovery",
              icon: faSearch,
              title: "Powerful Query Language",
              description:
                "Find exactly what you're looking for, fast. Search by metadata, filter by teams and explore your data landscape with an intuitive syntax.",
              features: [
                "Full-text search across all metadata",
                "Powerful built-in query language",
                "Save and share search queries",
              ],
              imageSrc: "/img/discovery.png",
              imageAlt: "Discovery interface",
            },
            {
              id: "lineage",
              label: "Lineage",
              icon: faProjectDiagram,
              title: "Lineage Visualization",
              description:
                "Understand data flow and dependencies with interactive lineage graphs. Track data throughout your entire stack, seeing exactly where your data is being used.",
              features: [
                "Impact analysis for data changes",
                "Data flow visualization",
                "OpenLineage support",
              ],
              imageSrc: "/img/lineage.png",
              imageAlt: "Lineage visualization",
            },
            {
              id: "metrics",
              label: "Metrics",
              icon: faChartLine,
              title: "Usage Analytics & Metrics",
              description:
                "Gain insights into how your data assets are being used. Track popularity and understand technology usage.",
              features: [
                "Asset trends and popularity",
                "Overview of your data landscape",
                "Technology usage insights",
              ],
              imageSrc: "/img/metrics.png",
              imageAlt: "Usage metrics and analytics",
            },
          ]}
        />
        <Integrations />
      </div>
    </Layout>
  );
}
