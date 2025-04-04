import React, { useEffect } from "react";
import clsx from "clsx";
import Link from "@docusaurus/Link";
import useDocusaurusContext from "@docusaurus/useDocusaurusContext";
import Layout from "@theme/Layout";
import styles from "./index.module.css";
import Hero from "../components/Hero";
import Query from "../components/Query";
import GetStarted from "../components/GetStarted";
import Lineage from "../components/Lineage";
import Integrations from "../components/Integrations";

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
        <Query />
        <Lineage />
        <Integrations />
      </div>
    </Layout>
  );
}
