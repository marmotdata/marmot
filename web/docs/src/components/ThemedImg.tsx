import React from "react";

interface ThemedImgProps {
  lightSrc: string;
  darkSrc: string;
  alt: string;
  className?: string;
}

export function ThemedImg({
  lightSrc,
  darkSrc,
  alt,
  className = "",
}: ThemedImgProps): JSX.Element {
  return (
    <>
      <img
        src={lightSrc}
        alt={alt}
        className={`dark:hidden ${className}`}
      />
      <img
        src={darkSrc}
        alt={alt}
        className={`hidden dark:block ${className}`}
      />
    </>
  );
}
