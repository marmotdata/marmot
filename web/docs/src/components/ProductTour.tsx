import React from "react";

export default function ProductTour(): JSX.Element {
  return (
    <section className="py-20 sm:py-24 px-4 sm:px-6 lg:px-8 bg-white dark:bg-gray-800">
      <div className="max-w-3xl mx-auto">
        <div data-animate className="text-center">
          <h2 className="text-3xl sm:text-4xl font-bold text-gray-900 dark:text-white tracking-tight">
            Not just for agents
          </h2>
          <p className="mt-4 text-lg text-gray-500 dark:text-gray-400">
            The catalog your assistants read is the one your team searches,
            browses and edits.
          </p>
        </div>

        <div data-animate data-animate-delay="1" className="mt-10">
          <iframe
            width="100%"
            height="100%"
            style={{ aspectRatio: "16 / 9", border: "none" }}
            className="rounded-xl shadow-lg"
            src="https://www.youtube.com/embed/_JBcQGj_bFU"
            title="Marmot demo"
            allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
            allowFullScreen
          ></iframe>
        </div>
      </div>
    </section>
  );
}
