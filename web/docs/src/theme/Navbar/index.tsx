import React from "react";
import clsx from "clsx";
import Link from "@docusaurus/Link";
import { useLocation } from "@docusaurus/router";
import { useColorMode } from "@docusaurus/theme-common";
import { useNavbarMobileSidebar } from "@docusaurus/theme-common/internal";
import NavbarMobileSidebar from "@theme/Navbar/MobileSidebar";
import SearchBar from "@theme/SearchBar";
import Wordmark from "@marmotdata/tokens/logos/marmot-wordmark.svg";
import MenuIcon from "~icons/material-symbols/menu";
import OpenInNew from "~icons/material-symbols/open-in-new";
import DarkMode from "~icons/material-symbols/dark-mode";
import DarkModeOutline from "~icons/material-symbols/dark-mode-outline";
import GitHub from "~icons/simple-icons/github";
import { CONSOLE_URL, useCloudSignedIn } from "@site/src/hooks/useCloudSignedIn";

// The same places as themeConfig.navbar.items, which the narrow-screen sidebar still draws.
const links: { href: string; label: string; external?: boolean }[] = [
  { href: "/docs/introduction", label: "Docs" },
  { href: "https://plugins.marmotdata.io", label: "Plugins" },
  { href: "/pricing", label: "Pricing" },
  { href: "/resources", label: "Resources" },
  { href: "/blog", label: "Blog" },
  { href: "https://discord.gg/TWCk7hVFN4", label: "Community", external: true },
];

/**
 * The site header, from the design system's marketing classes. Docusaurus's own pieces stay where
 * the docs rely on them: search, and on narrow screens its sidebar, which carries the docs
 * navigation as well as these links.
 */
export default function Navbar(): JSX.Element {
  const { pathname } = useLocation();
  const { colorMode, setColorMode } = useColorMode();
  const mobileSidebar = useNavbarMobileSidebar();
  const signedIn = useCloudSignedIn();
  const dark = colorMode === "dark";

  return (
    <header className={clsx("mkt-header", mobileSidebar.shown && "navbar-sidebar--show")}>
      {/* Docusaurus measures `.navbar` to offset the table of contents' highlight; this gives it the
          header's height without the old navbar's styles. */}
      <span className="navbar mkt-header__probe" aria-hidden="true" />
      <div className="mkt-header__inner">
        <Link className="mkt-header__brand" to="/" aria-label="Marmot home">
          <span className="mm-wordmark">
            <Wordmark aria-hidden="true" />
          </span>
        </Link>
        <nav className="mkt-header__nav" aria-label="Main">
          {links.map((l) =>
            l.external ? (
              <a key={l.href} href={l.href} target="_blank" rel="noopener noreferrer">
                {l.label}
                <OpenInNew aria-hidden="true" />
                <span className="mm-visually-hidden">(opens in a new tab)</span>
              </a>
            ) : (
              <Link
                key={l.href}
                to={l.href}
                aria-current={!l.href.includes("#") && pathname.startsWith(l.href.replace(/\/introduction$/, "")) ? "page" : undefined}
              >
                {l.label}
              </Link>
            ),
          )}
        </nav>
        <div className="mkt-header__end">
          <div className="mkt-header__wide">
            <SearchBar />
          </div>
          <a
            className="mm-btn mm-btn--ghost mm-btn--icon mkt-header__wide"
            href="https://github.com/marmotdata/marmot"
            target="_blank"
            rel="noopener noreferrer"
            aria-label="Marmot on GitHub (opens in a new tab)"
          >
            <GitHub aria-hidden="true" />
          </a>
          <button
            type="button"
            className="mm-btn mm-btn--ghost mm-btn--icon"
            aria-label="Dark theme"
            aria-pressed={dark}
            onClick={() => setColorMode(dark ? "light" : "dark")}
          >
            {dark ? <DarkMode aria-hidden="true" /> : <DarkModeOutline aria-hidden="true" />}
          </button>
          {signedIn ? (
            <a className="mm-btn mm-btn--primary" href={CONSOLE_URL}>
              Open console
            </a>
          ) : (
            <>
              <a className="mm-btn mm-btn--ghost mkt-header__wide" href="https://cloud.marmotdata.io/login">
                Sign in
              </a>
              <a className="mm-btn mm-btn--primary" href="https://cloud.marmotdata.io/signup">
                Start for free
              </a>
            </>
          )}
          <button
            type="button"
            className="mm-btn mm-btn--ghost mm-btn--icon mkt-header__narrow"
            aria-label="Menu"
            aria-expanded={mobileSidebar.shown}
            onClick={mobileSidebar.toggle}
          >
            <MenuIcon aria-hidden="true" />
          </button>
        </div>
      </div>
      <div role="presentation" className="navbar-sidebar__backdrop" onClick={mobileSidebar.toggle} />
      <NavbarMobileSidebar />
    </header>
  );
}
