import React from "react";
import IconAPI from "~icons/material-symbols/api-rounded";
import IconTerminal from "~icons/material-symbols/terminal";

interface DeployOption {
  title: string;
  description: string;
  href: string;
  icon: React.ReactNode;
}

const options: DeployOption[] = [
  {
    title: "Terraform",
    description:
      "Define your catalog as code alongside the rest of your infrastructure",
    href: "/docs/populating/terraform",
    icon: (
      <img
        src="/img/terraform.svg"
        alt="Terraform"
        className="w-8 h-8 object-contain"
      />
    ),
  },
  {
    title: "Pulumi",
    description: "Use TypeScript, Python, Go, or any language Pulumi supports",
    href: "/docs/populating/pulumi",
    icon: (
      <img
        src="/img/pulumi.svg"
        alt="Pulumi"
        className="w-8 h-8 object-contain"
      />
    ),
  },
  {
    title: "API",
    description: "REST API for every operation — integrate with anything",
    href: "/docs/populating/api",
    icon: <IconAPI className="w-8 h-8" />,
  },
  {
    title: "CLI",
    description:
      "Populate your catalog from scripts, CI pipelines, or your terminal",
    href: "/docs/populating/cli",
    icon: <IconTerminal className="w-8 h-8" />,
  },
];

export default function Integrations(): JSX.Element {
  return (
    <section className="py-16 px-4 sm:px-6 lg:px-8 bg-earthy-brown-50 dark:bg-gray-900">
      <div className="max-w-4xl mx-auto">
        <div data-animate className="text-center mb-10">
          <h2 className="text-2xl sm:text-3xl font-extrabold text-gray-900 dark:text-white mb-3 tracking-tight">
            Populate your way
          </h2>
          <p className="text-base text-gray-500 dark:text-gray-400 max-w-xl mx-auto">
            Infrastructure as code, REST API, or CLI - choose whatever fits your
            workflow.
          </p>
        </div>
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4">
          {options.map((opt, index) => (
            <a
              key={opt.title}
              href={opt.href}
              data-animate
              data-animate-delay={(index + 1).toString()}
              className="group flex flex-col items-center text-center p-5 rounded-xl border border-white/40 bg-white/60 backdrop-blur-md hover:bg-white/90 hover:border-earthy-terracotta-200 hover:shadow-lg transition-all duration-300 hover:-translate-y-0.5 dark:bg-gray-800/50 dark:border-white/10 dark:hover:bg-gray-800/80 dark:hover:border-earthy-terracotta-700"
            >
              <div className="mb-3 transition-transform duration-300 group-hover:scale-110">
                {opt.icon}
              </div>
              <h3 className="text-sm font-semibold text-gray-900 dark:text-white mb-1">
                {opt.title}
              </h3>
              <p className="text-xs text-gray-500 dark:text-gray-400 leading-relaxed">
                {opt.description}
              </p>
            </a>
          ))}
        </div>
      </div>
    </section>
  );
}
