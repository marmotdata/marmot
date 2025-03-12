import { writable } from 'svelte/store';
import { browser } from '$app/environment';

type Theme = 'light' | 'dark' | 'auto';

function createThemeStore() {
  const { subscribe, set: internalSet } = writable<Theme>('auto');

  function set(theme: Theme) {
    internalSet(theme);
    if (browser) {
      localStorage.setItem('theme', theme);
    }
    updateTheme(theme);
  }

  function updateTheme(theme: Theme) {
    if (browser) {
      const isDark =
        theme === 'dark' ||
        (theme === 'auto' && window.matchMedia('(prefers-color-scheme: dark)').matches);
      document.documentElement.classList.toggle('dark', isDark);

      // Listen for system theme changes when in auto mode
      const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
      const handleChange = () => {
        const currentTheme = getCurrentTheme();
        if (currentTheme === 'auto') {
          updateTheme('auto');
        }
      };
      mediaQuery.addEventListener('change', handleChange);
    }
  }

  // Function to safely get the current theme
  function getCurrentTheme(): Theme {
    if (browser) {
      return (localStorage.getItem('theme') as Theme) || 'auto';
    }
    return 'auto'; // Default value on the server
  }

  return {
    subscribe,
    set,
    initialize: () => {
      const savedTheme = getCurrentTheme();
      set(savedTheme);
    },
    getCurrentTheme // Export the function
  };
}

export const theme = createThemeStore();
