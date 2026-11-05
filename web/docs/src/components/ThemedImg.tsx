import React from "react";

interface ThemedImgProps {
  lightSrc: string;
  darkSrc: string;
  alt: string;
  /**
   * How wide the screenshot may grow: "window" for a full-window capture, "panel" (the text column) for part of a page, "dialog" for a dialog or drawer. Cropped captures are taken at 2x, so the cap keeps their text close to the page's.
   */
  size?: "window" | "panel" | "dialog";
  /** Plain light and dark images with no frame, for artwork such as the mascot rather than a screenshot. */
  bare?: boolean;
  className?: string;
}

/** A light and dark screenshot pair in a frame, so a capture never blends into the page around it. */
export function ThemedImg({
  lightSrc,
  darkSrc,
  alt,
  size = "panel",
  bare = false,
  className = "",
}: ThemedImgProps): JSX.Element {
  if (bare) {
    return (
      <>
        <img src={lightSrc} alt={alt} className={`themed-img__light ${className}`} />
        <img src={darkSrc} alt={alt} className={`themed-img__dark ${className}`} />
      </>
    );
  }
  return (
    <figure className={`doc-shot doc-shot--${size} ${className}`}>
      <img src={lightSrc} alt={alt} className="doc-shot__light" />
      <img src={darkSrc} alt={alt} className="doc-shot__dark" />
    </figure>
  );
}
