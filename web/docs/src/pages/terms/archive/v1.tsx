import React from "react";
import Layout from "@theme/Layout";
import TermsV1 from "../../../components/legal/TermsV1";

export default function TermsArchiveV1(): JSX.Element {
  return (
    <Layout
      title="Terms of Service — v1 (archived)"
      description="Archived version 1.0 of the Marmot Data Terms of Service."
    >
      <div className="bg-earthy-brown-50 dark:bg-gray-900 min-h-screen">
        <article className="max-w-3xl mx-auto px-4 sm:px-6 lg:px-8 py-16 prose prose-sm dark:prose-invert prose-headings:font-bold prose-a:text-earthy-terracotta-700 dark:prose-a:text-earthy-terracotta-400">
          <div className="not-prose mb-6 rounded-md border border-earthy-terracotta-300 bg-earthy-brown-100 dark:border-earthy-terracotta-700 dark:bg-gray-800 p-4 text-sm">
            <p className="m-0 text-gray-800 dark:text-gray-200">
              <strong>Archived version.</strong> This is version 1.0 of the
              Marmot Data Terms of Service, retained for reference. The
              current Terms are available at{" "}
              <a
                href="/terms"
                className="underline text-earthy-terracotta-700 dark:text-earthy-terracotta-400"
              >
                marmotdata.io/terms
              </a>
              .
            </p>
          </div>
          <h1>Marmot Data — Terms of Service (v1, archived)</h1>
          <TermsV1 />
        </article>
      </div>
    </Layout>
  );
}
