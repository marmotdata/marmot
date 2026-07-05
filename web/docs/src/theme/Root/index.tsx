import React from "react";
import { useLocation } from "@docusaurus/router";
import { TabSyncProvider } from "../../components/Steps";

/**
 * Re-keying TabSyncProvider on `location.pathname` resets the tab-sync state
 * on every navigation, so a language pick on one page doesn't leak into
 * other pages. State within a page is shared across all Tabs blocks that
 * pass the same `groupId`.
 */
export default function Root({
  children,
}: {
  children: React.ReactNode;
}): JSX.Element {
  const { pathname } = useLocation();
  return (
    <TabSyncProvider resetKey={pathname}>
      {children}
    </TabSyncProvider>
  );
}
