import React from "react";
import { Icon } from "@iconify/react";

interface DocCardProps {
  title: string;
  description: string;
  href: string;
  icon: string;
}

export function DocCard({ title, description, href, icon }: DocCardProps): JSX.Element {
  return (
    <a
      href={href}
      className="group block p-5 bg-gray-50 dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 hover:border-[var(--ifm-color-primary)] dark:hover:border-[var(--ifm-color-primary)] hover:shadow-lg transition-all no-underline"
    >
      <div className="flex items-start gap-4">
        <div className="flex-shrink-0 p-2 bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-700 group-hover:border-[var(--ifm-color-primary)] transition-colors">
          <Icon icon={icon} className="w-6 h-6 text-[var(--ifm-color-primary)]" />
        </div>
        <div className="flex-1 min-w-0">
          <h3 className="text-base font-semibold text-gray-900 dark:text-white m-0 group-hover:text-[var(--ifm-color-primary)] transition-colors">
            {title}
          </h3>
          <p className="mt-1 text-sm text-gray-600 dark:text-gray-400 m-0">
            {description}
          </p>
        </div>
        <Icon
          icon="mdi:arrow-right"
          className="w-5 h-5 text-gray-400 group-hover:text-[var(--ifm-color-primary)] group-hover:translate-x-1 transition-all"
        />
      </div>
    </a>
  );
}

interface DocCardGridProps {
  children: React.ReactNode;
}

export function DocCardGrid({ children }: DocCardGridProps): JSX.Element {
  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 mt-4 mb-6">
      {children}
    </div>
  );
}

interface CalloutCardProps {
  title: string;
  description: string;
  href: string;
  buttonText: string;
  variant?: "primary" | "secondary" | "external";
  icon?: string;
}

export function CalloutCard({
  title,
  description,
  href,
  buttonText,
  variant = "primary",
  icon,
}: CalloutCardProps): JSX.Element {
  const isPrimary = variant === "primary";
  const isExternal = variant === "external";

  const containerClasses = isPrimary
    ? "bg-gradient-to-br from-[var(--ifm-color-primary)] to-[#b34822] text-white"
    : isExternal
      ? "bg-gradient-to-br from-[var(--ifm-color-primary)]/10 to-[#b34822]/10 border border-[var(--ifm-color-primary)]/20 dark:from-[var(--ifm-color-primary)]/20 dark:to-[#b34822]/20 dark:border-[var(--ifm-color-primary)]/30"
      : "bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700";

  const iconClasses = isPrimary
    ? "text-white/90"
    : "text-[var(--ifm-color-primary)]";

  const titleClasses = isPrimary
    ? "text-white"
    : "text-gray-900 dark:text-white";

  const descriptionClasses = isPrimary
    ? "text-white/90"
    : "text-gray-600 dark:text-gray-400";

  const buttonClasses = isPrimary
    ? "bg-white text-[var(--ifm-color-primary)] hover:bg-gray-100"
    : "bg-[var(--ifm-color-primary)] text-white hover:text-white";

  return (
    <div
      className={`relative overflow-hidden rounded-xl p-6 mt-6 mb-6 ${containerClasses}`}
    >
      <div className="relative z-10">
        <div className="flex items-start gap-3">
          {icon && (
            <Icon
              icon={icon}
              className={`w-8 h-8 flex-shrink-0 ${iconClasses}`}
            />
          )}
          <div>
            <h3
              className={`text-lg font-bold m-0 ${titleClasses}`}
            >
              {title}
            </h3>
            <p
              className={`mt-2 mb-4 text-sm ${descriptionClasses}`}
            >
              {description}
            </p>
            <a
              href={href}
              className={`inline-flex items-center gap-2 px-4 py-2 rounded-lg font-medium text-sm transition-all no-underline ${buttonClasses}`}
            >
              {buttonText}
              <Icon icon={isExternal ? "mdi:open-in-new" : "mdi:arrow-right"} className="w-4 h-4" />
            </a>
          </div>
        </div>
      </div>
      {isPrimary && (
        <div className="absolute top-0 right-0 w-32 h-32 bg-white/10 rounded-full -translate-y-1/2 translate-x-1/2" />
      )}
    </div>
  );
}

interface FeatureCardProps {
  title: string;
  description: string;
  icon: string;
}

export function FeatureCard({ title, description, icon }: FeatureCardProps): JSX.Element {
  return (
    <div className="p-5 bg-gray-50 dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700">
      <div className="flex items-start gap-4">
        <div className="flex-shrink-0 p-2 bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-700">
          <Icon icon={icon} className="w-6 h-6 text-[var(--ifm-color-primary)]" />
        </div>
        <div className="flex-1 min-w-0">
          <h3 className="text-base font-semibold text-gray-900 dark:text-white m-0">
            {title}
          </h3>
          <p className="mt-1 text-sm text-gray-600 dark:text-gray-400 m-0">
            {description}
          </p>
        </div>
      </div>
    </div>
  );
}

export function FeatureGrid({ children }: DocCardGridProps): JSX.Element {
  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 mt-4 mb-6">
      {children}
    </div>
  );
}
