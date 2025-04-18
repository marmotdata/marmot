import React from "react";
import FeaturedCard from "./FeaturedCard";
import IconAPI from "~icons/material-symbols/api-rounded";
import IconTerminal from "~icons/material-symbols/terminal";
import IconsScroll from "../components/IconsScroll";

export default function Integrations(): JSX.Element {
  return (
    <section className="py-20 px-4 sm:px-6 lg:px-8 bg-earthy-brown-50 dark:bg-gray-900">
      <div className="max-w-6xl mx-auto">
        <div className="text-center mb-12">
          <h2 className="text-4xl font-bold text-gray-900 dark:text-white mb-6">
            Flexible Integration Options
          </h2>
          <p className="text-xl text-gray-600 dark:text-gray-300 max-w-3xl mx-auto">
            Integrate with Marmot your way. The flexible API supports a diverse
            set of data sources and infrastructure-as-code tools, letting you
            deploy virtually any asset type imaginable.
          </p>
        </div>
        <div className="mt-12 grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
          {/* Example using different styling options */}

          {/* Option 1: Using individual colors for each card */}
          <FeaturedCard
            icon={IconTerminal}
            title="CLI"
            description="Quick integration via command line"
            href="/docs/populating/cli"
            color="bg-earthy-brown-50 dark:bg-gray-900"
            large={true}
          />

          <FeaturedCard
            icon={IconAPI}
            title="API"
            description="Integrate with anything using the API"
            href="/docs/populating/api"
            color="bg-earthy-brown-50 dark:bg-gray-900"
            large={true}
          />

          {/* Option 2: Using the same theme color for all cards */}
          <FeaturedCard
            icon="terraform"
            title="Terraform"
            description="Infrastructure as code support for all your resources"
            href="/docs/populating/terraform"
            color="bg-earthy-brown-50 dark:bg-gray-900"
            large={true}
          />

          <FeaturedCard
            icon="pulumi"
            title="Pulumi"
            description="Modern IaC with your favorite programming language"
            href="/docs/populating/pulumi"
            color="bg-earthy-brown-50 dark:bg-gray-900"
            large={true}
          />
        </div>
      </div>
    </section>
  );
}
