import React, { useEffect, useState } from 'react';
import Content from '@theme-original/CodeBlock/Content';
import type ContentType from '@theme/CodeBlock/Content';
import type { WrapperProps } from '@docusaurus/types';

type Props = WrapperProps<typeof ContentType>;

// Hook to observe data-theme attribute changes
function useTheme(): string {
  const [theme, setTheme] = useState<string>('light');

  useEffect(() => {
    const htmlElement = document.documentElement;
    const currentTheme = htmlElement.getAttribute('data-theme') || 'light';
    setTheme(currentTheme);

    const observer = new MutationObserver(() => {
      const newTheme = htmlElement.getAttribute('data-theme') || 'light';
      setTheme(newTheme);
    });

    observer.observe(htmlElement, {
      attributes: true,
      attributeFilter: ['data-theme'],
    });

    return () => observer.disconnect();
  }, []);

  return theme;
}

export default function ContentWrapper(props: Props): JSX.Element {
  const theme = useTheme();

  // Use theme as key to force Prism to re-render with correct theme
  return <Content key={theme} {...props} />;
}
