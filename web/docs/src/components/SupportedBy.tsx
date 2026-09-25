import React from "react";
import clsx from "clsx";

export default function SupportedBy({
  className,
  ...rest
}: React.ComponentProps<"div">): JSX.Element {
  return (
    <div
      data-animate
      data-animate-delay="4"
      className={clsx("supported-by", className)}
      {...rest}
    >
      <span className="supported-by-label">Part of</span>
      <div className="supported-by-logos">
        <img
          src="/img/programmes/nvidia-inception-transparent.svg"
          alt="NVIDIA Inception Program"
          className="supported-by-badge dark:hidden"
        />
        <img
          src="/img/programmes/nvidia-inception-transparent-dark.svg"
          alt="NVIDIA Inception Program"
          className="supported-by-badge hidden dark:inline"
        />
      </div>
    </div>
  );
}
