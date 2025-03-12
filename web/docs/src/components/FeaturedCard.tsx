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
  color = "bg-white dark:bg-gray-800",
  useThemeColor = false,
  themeColor = "bg-amber-50 dark:bg-amber-900/20",
  large = false,
  className = "",
}: FeaturedCardProps): JSX.Element {
  const CardContent = () => (
    <>
      <div className="mb-4 flex justify-center">
        <Icon
          type={icon}
          size={large ? "lg" : "md"}
          className={`transition-transform duration-300 group-hover:scale-110 ${
            large ? "w-20 h-20" : ""
          }`}
        />
      </div>
      <h3
        className={`${
          large ? "text-xl" : "text-lg"
        } font-medium text-gray-900 dark:text-white mb-2`}
      >
        {title}
      </h3>
      <p
        className={`${
          large ? "text-base" : "text-sm"
        } text-gray-600 dark:text-gray-300`}
      >
        {description}
      </p>
    </>
  );

  const classes = `
    ${useThemeColor ? themeColor : color}
    ${className}
    group
    ${large ? "p-8" : "p-6"}
    rounded-lg 
    border border-gray-200 dark:border-gray-700
    shadow-sm 
    hover:shadow-md 
    hover:border-amber-500 dark:hover:border-amber-400
    transition-all 
    duration-200 
    hover:translate-y-[-2px]
  `;

  if (href) {
    return (
      <a href={href} className={classes}>
        <CardContent />
      </a>
    );
  }

  return (
    <div className={classes}>
      <CardContent />
    </div>
  );
}
