import React from "react";
import clsx from "clsx";

type Testimonial = {
  quote: string;
  name: string;
  role: string;
  company: string;
  avatar?: string;
};

const testimonials: Testimonial[] = [
  {
    quote:
      "Marmot brings metadata, ownership and lineage into one searchable catalog, making data products easier to discover and understand.",
    name: "Manuel Jáñez García",
    role: "Senior Specialist of Data Governance Services",
    company: "Telefónica",
    avatar: "/img/testimonials/manuel-janez-garcia.jpg",
  },
];

export default function Testimonials(): JSX.Element {
  // A lone quote gets the full width; once there are more they sit side by side in a grid.
  const single = testimonials.length === 1;

  return (
    <section className="py-20 sm:py-24 px-4 sm:px-6 lg:px-8 bg-earthy-brown-50 dark:bg-gray-900 border-t border-dashed border-earthy-brown-200/70 dark:border-gray-800/70">
      <div
        className={clsx(
          "mx-auto",
          single
            ? "max-w-3xl"
            : "max-w-6xl grid gap-6 sm:grid-cols-2 lg:grid-cols-3",
        )}
      >
        {testimonials.map((t) => (
          <figure
            key={t.name}
            data-animate
            className={clsx(
              "m-0",
              single
                ? "text-center"
                : "flex flex-col justify-between rounded-2xl p-6 bg-white/70 dark:bg-gray-800/50 border border-earthy-brown-200/70 dark:border-gray-700/70",
            )}
          >
            <blockquote
              className={clsx(
                "m-0 p-0 border-0 text-gray-900 dark:text-white tracking-tight",
                single
                  ? "text-2xl sm:text-3xl font-semibold leading-snug"
                  : "text-lg font-medium leading-relaxed",
              )}
              style={single ? { textWrap: "balance" } : undefined}
            >
              <span className="text-earthy-terracotta-700 dark:text-earthy-terracotta-400">
                &ldquo;
              </span>
              {t.quote}
              <span className="text-earthy-terracotta-700 dark:text-earthy-terracotta-400">
                &rdquo;
              </span>
            </blockquote>
            <figcaption
              className={clsx(
                "flex items-center gap-3",
                single ? "mt-8 justify-center" : "mt-6",
              )}
            >
              {t.avatar && (
                <img
                  src={t.avatar}
                  alt=""
                  width={56}
                  height={56}
                  loading="lazy"
                  className="h-14 w-14 shrink-0 rounded-full object-cover ring-2 ring-white dark:ring-gray-800"
                />
              )}
              <div className="text-left">
                <div className="text-sm font-semibold text-gray-900 dark:text-white">
                  {t.name}
                </div>
                <div className="text-sm text-gray-500 dark:text-gray-400">
                  {t.role}
                </div>
                <div className="mt-0.5 text-sm font-semibold text-earthy-terracotta-700 dark:text-earthy-terracotta-400">
                  {t.company}
                </div>
              </div>
            </figcaption>
          </figure>
        ))}
      </div>
    </section>
  );
}
