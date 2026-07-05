import React from "react";
import Layout from "@theme-original/Layout";
import FloatingThemeToggle from "../../components/FloatingThemeToggle";

type Props = {
  children?: React.ReactNode;
  [key: string]: unknown;
};

/**
 * FloatingThemeToggle uses useColorMode, which needs the ColorModeProvider
 * that Layout sets up — it can't live in Root, which renders above it.
 */
export default function LayoutWrapper({ children, ...props }: Props): JSX.Element {
  return (
    <Layout {...props}>
      {children}
      <FloatingThemeToggle />
    </Layout>
  );
}
