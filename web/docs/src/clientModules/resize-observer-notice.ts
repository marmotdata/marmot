/**
 * The design system's data table lays its rows out with @lit-labs/virtualizer, which resizes them
 * from inside a ResizeObserver. Browsers then report "ResizeObserver loop completed with
 * undelivered notifications": a notice that some resize callbacks moved to the next frame, not a
 * failure, and visitors never see it. Docusaurus hard-codes webpack's dev overlay to treat every
 * runtime error as a crash, so in development this one notice is caught first (a capturing
 * listener on window runs before the overlay's) and nothing else is touched.
 */
if (process.env.NODE_ENV === "development" && typeof window !== "undefined") {
  window.addEventListener(
    "error",
    (event) => {
      if (event.message?.startsWith("ResizeObserver loop")) event.stopImmediatePropagation();
    },
    { capture: true },
  );
}

export {};
