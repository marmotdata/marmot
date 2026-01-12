import React, { useState, useEffect } from "react";
import Giscus from "@giscus/react";

const GISCUS_LIGHT_THEME = "noborder_light";
const GISCUS_DARK_THEME = "noborder_dark";

export default function GiscusComment(): JSX.Element {
  const [theme, setTheme] = useState<string>(GISCUS_LIGHT_THEME);

  useEffect(() => {
    // Get initial theme
    const currentTheme = document.documentElement.getAttribute("data-theme");
    setTheme(currentTheme === "dark" ? GISCUS_DARK_THEME : GISCUS_LIGHT_THEME);

    // Watch for theme changes
    const observer = new MutationObserver((mutations) => {
      mutations.forEach((mutation) => {
        if (mutation.attributeName === "data-theme") {
          const newTheme = document.documentElement.getAttribute("data-theme");
          setTheme(
            newTheme === "dark" ? GISCUS_DARK_THEME : GISCUS_LIGHT_THEME
          );
        }
      });
    });

    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-theme"],
    });

    return () => observer.disconnect();
  }, []);

  return (
    <div className="docusaurus-mt-lg">
      <Giscus
        key={theme}
        repo="marmotdata/marmot"
        repoId="R_kgDOOHls4w"
        category="Announcements"
        categoryId="DIC_kwDOOHls484Czp9j"
        mapping="pathname"
        strict="0"
        reactionsEnabled="1"
        emitMetadata="0"
        inputPosition="bottom"
        theme={theme}
        lang="en"
      />
    </div>
  );
}
