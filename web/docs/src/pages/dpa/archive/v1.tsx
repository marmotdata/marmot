import React from "react";
import Layout from "@theme/Layout";
import DPAV1 from "../../../components/legal/DPAV1";

export default function DPAArchiveV1(): JSX.Element {
  return (
    <Layout
      title="Data Processing Agreement — v1 (archived)"
      description="Archived version 1.0 of the Marmot Data Data Processing Agreement."
    >
      <div className="bg-earthy-brown-50 dark:bg-gray-900 min-h-screen">
        <article className="max-w-3xl mx-auto px-4 sm:px-6 lg:px-8 py-16 prose prose-sm dark:prose-invert prose-headings:font-bold prose-a:text-earthy-terracotta-700 dark:prose-a:text-earthy-terracotta-400">
          <div className="not-prose mb-6 rounded-md border border-earthy-terracotta-300 bg-earthy-brown-100 dark:border-earthy-terracotta-700 dark:bg-gray-800 p-4 text-sm">
            <p className="m-0 text-gray-800 dark:text-gray-200">
              <strong>Archived version.</strong> This is version 1.0 of the
              Marmot Data Data Processing Agreement, retained for reference.
              The current DPA is available at{" "}
              <a
                href="/dpa"
                className="underline text-earthy-terracotta-700 dark:text-earthy-terracotta-400"
              >
                marmotdata.io/dpa
              </a>
              .
            </p>
          </div>
          <h1>Data Processing Agreement (v1, archived)</h1>
          <DPAV1 />
        </article>
      </div>
    </Layout>
  );
}
