import React, { useEffect, useState } from "react";
import { Icon } from "@iconify/react";

export default function FloatingThemeToggle(): JSX.Element {
  const [theme, setTheme] = useState<string>("light");

  useEffect(() => {
    // Get initial theme from HTML element
    const htmlElement = document.documentElement;
    const currentTheme = htmlElement.getAttribute("data-theme") || "light";
    setTheme(currentTheme);

    // Listen for theme changes
    const observer = new MutationObserver(() => {
      const newTheme = htmlElement.getAttribute("data-theme") || "light";
      setTheme(newTheme);
    });

    observer.observe(htmlElement, {
      attributes: true,
      attributeFilter: ["data-theme"],
    });

    return () => observer.disconnect();
  }, []);

  const toggleColorMode = () => {
    const htmlElement = document.documentElement;
    const newTheme = theme === "dark" ? "light" : "dark";
    htmlElement.setAttribute("data-theme", newTheme);
    setTheme(newTheme);
    // Store preference
    localStorage.setItem("theme", newTheme);
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
      title={`Switch to ${theme === "dark" ? "light" : "dark"} mode`}
    >
      {theme === "dark" ? (
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
