import React, { useCallback } from "react";
import clsx from "clsx";
import { useHistory, useLocation } from "@docusaurus/router";
import { useColorMode } from "@docusaurus/theme-common";
import { useNavbarMobileSidebar } from "@docusaurus/theme-common/internal";
import NavbarMobileSidebar from "@theme/Navbar/MobileSidebar";
import SearchBar from "@theme/SearchBar";
import { MmSiteHeader } from "@marmotdata/react/site-header.js";
import { CONSOLE_URL, useCloudSignedIn } from "@site/src/hooks/useCloudSignedIn";

const SITE = "https://marmotdata.io";

// The header's sections by the first segment of the path, so each page marks the one it belongs to.
const sections: [string, string][] = [
  ["/docs", "Docs"],
  ["/pricing", "Pricing"],
  ["/resources", "Resources"],
  ["/blog", "Blog"],
];

function currentSection(pathname: string): string {
  const hit = sections.find(([prefix]) => pathname === prefix || pathname.startsWith(`${prefix}/`));
  return hit ? hit[1] : "";
}

/**
 * The site header is the design system's mm-site-header, the same header as the plugin directory and Marmot Cloud's public pages. Docusaurus keeps the parts the docs rely on: its search in the header's search slot, its colour mode for the theme button and on narrow screens its sidebar, which carries the docs navigation as well as these links.
 */
export default function Navbar(): React.JSX.Element {
  const { pathname } = useLocation();
  const history = useHistory();
  const { setColorMode } = useColorMode();
  const mobileSidebar = useNavbarMobileSidebar();
  const signedIn = useCloudSignedIn();

  const onThemeChange = useCallback(
    (e: CustomEvent<{ theme: "light" | "dark" }>) => {
      e.preventDefault();
      setColorMode(e.detail.theme);
    },
    [setColorMode],
  );

  // Docusaurus's sidebar only renders at its own mobile width; wider than that (a row that does not fit) the header's own menu panel opens instead.
  const onMenuToggle = useCallback(
    (e: CustomEvent<{ open: boolean }>) => {
      if (!mobileSidebar.shouldRender) return;
      e.preventDefault();
      mobileSidebar.toggle();
    },
    [mobileSidebar],
  );

  // The header's links are absolute marmotdata.io URLs so they work on every site; here they route client-side.
  const onNavigate = useCallback(
    (e: CustomEvent<{ href: string }>) => {
      const { href } = e.detail;
      if (href !== SITE && !href.startsWith(`${SITE}/`)) return;
      e.preventDefault();
      history.push(href.slice(SITE.length) || "/");
    },
    [history],
  );

  return (
    <div className={clsx("site-header", mobileSidebar.shown && "navbar-sidebar--show")}>
      {/* Docusaurus measures `.navbar` to offset the table of contents' highlight; this gives it the header's height without the old navbar's styles. */}
      <span className="navbar site-header__probe" aria-hidden="true" />
      <MmSiteHeader
        current={currentSection(pathname)}
        signedIn={signedIn}
        consoleHref={CONSOLE_URL}
        navExpanded={mobileSidebar.shown}
        onMmThemeChange={onThemeChange}
        onMmMenuToggle={onMenuToggle}
        onMmNavigate={onNavigate}
      >
        <div slot="search" className="site-header__search">
          <SearchBar />
        </div>
      </MmSiteHeader>
      <div role="presentation" className="navbar-sidebar__backdrop" onClick={mobileSidebar.toggle} />
      <NavbarMobileSidebar />
    </div>
  );
}
