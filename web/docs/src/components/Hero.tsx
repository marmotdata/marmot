import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";
import { faExternalLinkAlt } from "@fortawesome/free-solid-svg-icons";
import React from "react";

export default function Hero(): JSX.Element {
  return (
    <header className="py-16 px-4 sm:px-6 lg:px-8 bg-earthy-brown-50 dark:bg-gray-900">
      <div className="max-w-7xl mx-auto text-center">
        <img
          src="/img/marmot.svg"
          alt={`Marmot logo`}
          className="max-w-[12rem]"
        />
        <h1 className="text-5xl font-bold text-gray-900 dark:text-white mb-6">
          Modern Data Discovery for Modern Teams
        </h1>
        <p className="text-xl text-gray-600 dark:text-gray-300 max-w-3xl mx-auto">
          Marmot helps teams discover, understand, and leverage their data with
          powerful search and lineage visualization tools. It's designed to make
          data accessible for everyone.
        </p>
        <div className="mt-8 flex justify-center gap-4">
          <a
            href="/docs/introduction"
            className="inline-flex items-center px-6 py-3 border border-transparent text-base font-medium rounded-md text-white bg-amber-600 hover:bg-amber-700"
          >
            Get Started
          </a>
          <a
            href="https://github.com/marmotdata/marmot"
            className="inline-flex items-center px-6 py-3 border border-gray-300 dark:border-gray-600 text-base font-medium rounded-md text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-800 hover:bg-gray-50 dark:hover:bg-gray-700"
            target="_blank"
            rel="noopener noreferrer"
          >
            View on GitHub
            <FontAwesomeIcon icon={faExternalLinkAlt} className="ml-2" />
          </a>
        </div>
      </div>
    </header>
  );
}
