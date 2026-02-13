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
    title: "CLI",
    description: "Quick integration via command line",
    href: "/docs/populating/cli",
    icon: <IconTerminal className="w-10 h-10" />,
  },
  {
    title: "API",
    description: "Integrate with anything using the API",
    href: "/docs/populating/api",
    icon: <IconAPI className="w-10 h-10" />,
  },
  {
    title: "Terraform",
    description: "Infrastructure as code support for all your resources",
    href: "/docs/populating/terraform",
    icon: (
      <img
        src="/img/terraform.svg"
        alt="Terraform"
        className="w-10 h-10 object-contain"
      />
    ),
  },
  {
    title: "Pulumi",
    description: "Modern IaC with your favorite programming language",
    href: "/docs/populating/pulumi",
    icon: (
      <img
        src="/img/pulumi.svg"
        alt="Pulumi"
        className="w-10 h-10 object-contain"
      />
    ),
  },
];

export default function Integrations(): JSX.Element {
  return (
    <section className="py-24 px-4 sm:px-6 lg:px-8 bg-white dark:bg-gray-800">
      <div className="max-w-5xl mx-auto">
        <div data-animate className="text-center mb-14">
          <h2 className="text-3xl sm:text-4xl font-extrabold text-gray-900 dark:text-white mb-4 tracking-tight">
            Populate your way
          </h2>
          <p className="text-lg text-gray-500 dark:text-gray-400 max-w-2xl mx-auto">
            Use CLI, API, Terraform, or Pulumi to manage your data catalog as
            code
          </p>
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-5">
          {options.map((opt, index) => (
            <a
              key={opt.title}
              href={opt.href}
              data-animate
              data-animate-delay={(index + 1).toString()}
              className="group flex flex-col items-center text-center p-8 rounded-2xl border border-white/40 bg-white/60 backdrop-blur-md hover:bg-white/90 hover:border-earthy-terracotta-200 hover:shadow-lg transition-all duration-300 hover:-translate-y-0.5 dark:bg-gray-800/50 dark:border-white/10 dark:hover:bg-gray-800/80 dark:hover:border-earthy-terracotta-700"
            >
              <div className="mb-5 transition-transform duration-300 group-hover:scale-110">
                {opt.icon}
              </div>
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-2">
                {opt.title}
              </h3>
              <p className="text-sm text-gray-500 dark:text-gray-400 leading-relaxed">
                {opt.description}
              </p>
            </a>
          ))}
        </div>
      </div>
    </section>
  );
}
