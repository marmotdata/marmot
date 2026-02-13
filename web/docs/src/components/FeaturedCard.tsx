import React from "react";
import Icon from "./Icon";

interface FeaturedCardProps {
  icon: string | any;
  title: string;
  description: string;
  href?: string;
  color?: string;
  useThemeColor?: boolean;
  themeColor?: string;
  large?: boolean;
  className?: string;
}

export default function FeaturedCard({
  icon,
  title,
  description,
  href,
  large = false,
  className = "",
}: FeaturedCardProps): JSX.Element {
  const CardContent = () => (
    <>
      <div className="mb-5 flex justify-center">
        <Icon
          type={icon}
          size={large ? "lg" : "md"}
          className="transition-transform duration-300 group-hover:scale-110"
        />
      </div>
      <h3
        className={`${large ? "text-lg" : "text-base"} font-semibold text-gray-900 dark:text-white mb-2`}
      >
        {title}
      </h3>
      <p
        className={`${large ? "text-sm" : "text-xs"} text-gray-500 dark:text-gray-400 leading-relaxed`}
      >
        {description}
      </p>
    </>
  );

  const padding = large ? "p-8" : "p-6";
  const base = `group ${padding} rounded-2xl text-center h-full bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 hover:border-earthy-terracotta-300 dark:hover:border-earthy-terracotta-700 hover:shadow-lg transition-all duration-300 hover:-translate-y-0.5 ${className}`;

  if (href) {
    return (
      <a href={href} className={base}>
        <CardContent />
      </a>
    );
  }

  return (
    <div className={base}>
      <CardContent />
    </div>
  );
}
