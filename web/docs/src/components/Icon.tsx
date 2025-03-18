import React from "react";

interface IconProps {
  type: string | React.ComponentType<any> | any;
  size?: "sm" | "md" | "lg";
  className?: string;
}

export default function Icon({
  type,
  size = "md",
  className = "",
}: IconProps): JSX.Element {
  // Size mappings for both container and icon
  const sizeClasses = {
    sm: {
      container: "w-8 h-8",
      icon: "w-5 h-5",
    },
    md: {
      container: "w-12 h-12",
      icon: "w-8 h-8",
    },
    lg: {
      container: "w-16 h-16",
      icon: "w-10 h-10",
    },
  };

  // Container
  return (
    <div
      className={`
        ${sizeClasses[size].container}
        flex
        items-center
        justify-center
        p-1
        ${className}
      `}
    >
      {typeof type === "string" ? (
        // String case - render image
        <img
          src={`/img/${type.toLowerCase()}.svg`}
          alt={`${type} icon`}
          className={`${sizeClasses[size].icon} object-contain`}
        />
      ) : typeof type === "object" &&
        type !== null &&
        typeof type.$$typeof === "symbol" &&
        type.render ? (
        // Handle Unplugin icons (which are forward refs with a render method)
        React.createElement(type, { className: sizeClasses[size].icon })
      ) : typeof type === "function" ? (
        // Regular function component
        React.createElement(type, { className: sizeClasses[size].icon })
      ) : (
        // Default fallback
        <div
          className={`${sizeClasses[size].icon} flex items-center justify-center text-gray-400`}
        >
          ?
        </div>
      )}
    </div>
  );
}
