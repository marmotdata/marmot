const colors = require("tailwindcss/colors");
const plugin = require("tailwindcss/plugin");

/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./src/**/*.{js,jsx,ts,tsx}"],
  darkMode: ["class", '[data-theme="dark"]'],
  theme: {
    extend: {
      colors: {
        amber: colors.amber,
        gray: {
          50: "#F9F9F9",
          100: "#ECECEC",
          200: "#DFDFDF",
          300: "#CCCCCC",
          400: "#B0B0B0",
          500: "#8F8F8F",
          600: "#696969",
          700: "#4D4D4D",
          800: "#2E2E2E",
          900: "#1A1A1A",
        },
      },
    },
  },
  plugins: [require("@tailwindcss/typography")],
  corePlugins: {
    preflight: false,
  },
};
