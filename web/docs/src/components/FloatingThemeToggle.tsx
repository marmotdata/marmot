import React from "react";
import { Icon } from "@iconify/react";
import { useColorMode } from "@docusaurus/theme-common";

export default function FloatingThemeToggle(): JSX.Element {
  const { colorMode, setColorMode } = useColorMode();

  const toggleColorMode = () => {
    setColorMode(colorMode === "dark" ? "light" : "dark");
  };

  return (
    <a
      href="#"
      onClick={(e) => {
        e.preventDefault();
        toggleColorMode();
      }}
      className="fixed bottom-8 right-8 z-50 p-4 rounded-2xl shadow-xl bg-gradient-to-br from-white to-gray-50 dark:from-gray-800 dark:to-gray-900 border border-gray-200 dark:border-gray-700 hover:shadow-2xl hover:scale-105 active:scale-95 transition-all duration-300 backdrop-blur-sm inline-flex items-center justify-center"
      aria-label="Toggle theme"
      title={`Switch to ${colorMode === "dark" ? "light" : "dark"} mode`}
    >
      {colorMode === "dark" ? (
        <Icon
          icon="mdi:white-balance-sunny"
          className="w-7 h-7 text-yellow-400 drop-shadow-md"
        />
      ) : (
        <Icon
          icon="mdi:moon-waning-crescent"
          className="w-7 h-7 text-indigo-600 dark:text-indigo-400 drop-shadow-md"
        />
      )}
    </a>
  );
}
