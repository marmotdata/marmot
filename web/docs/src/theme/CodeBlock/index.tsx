import React, { useEffect, useState, type ReactNode } from 'react';
import CodeBlock from '@theme-original/CodeBlock';
import type CodeBlockType from '@theme/CodeBlock';
import type { WrapperProps } from '@docusaurus/types';

type Props = WrapperProps<typeof CodeBlockType>;

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

export default function CodeBlockWrapper(props: Props): JSX.Element {
  const theme = useTheme();

  // Force complete remount of CodeBlock when theme changes
  // by using theme in the key
  return <CodeBlock key={`codeblock-${theme}`} {...props} />;
}
