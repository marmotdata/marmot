import React from "react";
import Layout from "@theme/Layout";
import TermsV1 from "../components/legal/TermsV1";

export default function Terms(): JSX.Element {
  return (
    <Layout
      title="Terms of Service"
      description="Marmot Data Terms of Service — the agreement between Marmot Data Ltd and organisations that use the Service."
    >
      <div className="bg-earthy-brown-50 dark:bg-gray-900 min-h-screen">
        <article className="max-w-3xl mx-auto px-4 sm:px-6 lg:px-8 py-16 prose prose-sm dark:prose-invert prose-headings:font-bold prose-a:text-earthy-terracotta-700 dark:prose-a:text-earthy-terracotta-400">
          <h1>Marmot Data — Terms of Service</h1>
          <TermsV1 />
          <p className="text-gray-500 dark:text-gray-400 text-xs">
            Previous versions of these Terms are available in the{" "}
            <a href="/terms/archive/v1">archive</a>.
          </p>
        </article>
      </div>
    </Layout>
  );
}
