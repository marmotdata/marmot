/**
 * Keyboard navigation utilities for dropdown/search components
 */

export interface KeyboardNavigationOptions<T> {
	/** The list of items to navigate through */
	items: T[];
	/** Current selected index (-1 means nothing selected) */
	selectedIndex: number;
	/** Callback when selected index changes */
	onIndexChange: (index: number) => void;
	/** Callback when an item is selected (Enter pressed) */
	onSelect?: (item: T) => void;
	/** Callback when Escape is pressed */
	onEscape?: () => void;
	/** Whether to wrap around when reaching the end/beginning */
	wrap?: boolean;
}

/**
 * Creates a keydown handler for keyboard navigation in dropdown/search components.
 *
 * Supports:
 * - ArrowDown: Move selection down
 * - ArrowUp: Move selection up
 * - Enter: Select current item
 * - Escape: Clear/close
 *
 * @example
 * ```svelte
 * <script>
 *   let selectedIndex = $state(-1);
 *   const handleKeydown = createKeyboardNavigation({
 *     items: searchResults,
 *     selectedIndex,
 *     onIndexChange: (i) => selectedIndex = i,
 *     onSelect: (item) => selectItem(item),
 *     onEscape: () => { query = ''; results = []; }
 *   });
 * </script>
 *
 * <input onkeydown={handleKeydown} />
 * ```
 */
export function createKeyboardNavigation<T>(
	options: KeyboardNavigationOptions<T>
): (event: KeyboardEvent) => void {
	return (event: KeyboardEvent) => {
		const { items, selectedIndex, onIndexChange, onSelect, onEscape, wrap = false } = options;

		switch (event.key) {
			case 'ArrowDown':
				event.preventDefault();
				if (items.length === 0) return;
				if (wrap) {
					onIndexChange((selectedIndex + 1) % items.length);
				} else {
					onIndexChange(Math.min(selectedIndex + 1, items.length - 1));
				}
				break;

			case 'ArrowUp':
				event.preventDefault();
				if (items.length === 0) return;
				if (wrap) {
					onIndexChange(selectedIndex <= 0 ? items.length - 1 : selectedIndex - 1);
				} else {
					onIndexChange(Math.max(selectedIndex - 1, -1));
				}
				break;

			case 'Enter':
				if (selectedIndex >= 0 && selectedIndex < items.length && onSelect) {
					event.preventDefault();
					onSelect(items[selectedIndex]);
				}
				break;

			case 'Escape':
				if (onEscape) {
					event.preventDefault();
					onEscape();
				}
				break;
		}
	};
}

/**
 * Reactive keyboard navigation hook for Svelte 5.
 * Returns an object with state and handlers that can be used directly.
 *
 * @example
 * ```svelte
 * <script>
 *   let results = $state([]);
 *   const nav = useKeyboardNavigation({
 *     getItems: () => results,
 *     onSelect: (item) => selectItem(item),
 *     onEscape: () => clearSearch()
 *   });
 * </script>
 *
 * <input onkeydown={nav.handleKeydown} />
 * {#each results as item, i}
 *   <div class={i === nav.selectedIndex ? 'selected' : ''}>
 *     {item.name}
 *   </div>
 * {/each}
 * ```
 */
export interface UseKeyboardNavigationOptions<T> {
	/** Function that returns the current items (for reactivity) */
	getItems: () => T[];
	/** Callback when an item is selected */
	onSelect?: (item: T) => void;
	/** Callback when Escape is pressed */
	onEscape?: () => void;
	/** Whether to wrap around */
	wrap?: boolean;
}

export interface KeyboardNavigationState {
	/** Current selected index */
	selectedIndex: number;
	/** Reset selected index to -1 */
	reset: () => void;
	/** Set selected index */
	setIndex: (index: number) => void;
	/** Keydown event handler */
	handleKeydown: (event: KeyboardEvent) => void;
}

/**
 * Creates a keyboard navigation state object.
 * Note: In Svelte 5, you'll need to use $state for the index in the component.
 */
export function createKeyboardNavigationState<T>(
	getItems: () => T[],
	getSelectedIndex: () => number,
	setSelectedIndex: (index: number) => void,
	options: {
		onSelect?: (item: T) => void;
		onEscape?: () => void;
		wrap?: boolean;
	} = {}
): { handleKeydown: (event: KeyboardEvent) => void } {
	const { onSelect, onEscape, wrap = false } = options;

	const handleKeydown = (event: KeyboardEvent) => {
		const items = getItems();
		const selectedIndex = getSelectedIndex();

		switch (event.key) {
			case 'ArrowDown':
				event.preventDefault();
				if (items.length === 0) return;
				if (wrap) {
					setSelectedIndex((selectedIndex + 1) % items.length);
				} else {
					setSelectedIndex(Math.min(selectedIndex + 1, items.length - 1));
				}
				break;

			case 'ArrowUp':
				event.preventDefault();
				if (items.length === 0) return;
				if (wrap) {
					setSelectedIndex(selectedIndex <= 0 ? items.length - 1 : selectedIndex - 1);
				} else {
					setSelectedIndex(Math.max(selectedIndex - 1, -1));
				}
				break;

			case 'Enter':
				if (selectedIndex >= 0 && selectedIndex < items.length && onSelect) {
					event.preventDefault();
					onSelect(items[selectedIndex]);
				}
				break;

			case 'Escape':
				if (onEscape) {
					event.preventDefault();
					onEscape();
				}
				break;
		}
	};

	return { handleKeydown };
}
