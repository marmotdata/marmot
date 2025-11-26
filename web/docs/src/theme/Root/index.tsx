import React from "react";
import FloatingThemeToggle from "../../components/FloatingThemeToggle";

export default function Root({ children }: { children: React.ReactNode }): JSX.Element {
  return (
    <>
      {children}
      <FloatingThemeToggle />
    </>
  );
}
