import React from "react";
import Layout from "@theme/Layout";
import DPAV1 from "../components/legal/DPAV1";

export default function DPA(): JSX.Element {
  return (
    <Layout
      title="Data Processing Agreement"
      description="Marmot Data — Data Processing Agreement between Marmot Data Ltd (Processor) and the Controller."
    >
      <div className="bg-earthy-brown-50 dark:bg-gray-900 min-h-screen">
        <article className="max-w-3xl mx-auto px-4 sm:px-6 lg:px-8 py-16 prose prose-sm dark:prose-invert prose-headings:font-bold prose-a:text-earthy-terracotta-700 dark:prose-a:text-earthy-terracotta-400">
          <h1>Data Processing Agreement</h1>
          <DPAV1 />
          <p className="text-gray-500 dark:text-gray-400 text-xs">
            Previous versions of this DPA are available in the{" "}
            <a href="/dpa/archive/v1">archive</a>.
          </p>
        </article>
      </div>
    </Layout>
  );
}
