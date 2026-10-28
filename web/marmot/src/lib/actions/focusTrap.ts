// Shared focus management for dialog-like panels (Blade, Modal): moves focus into the panel on
// mount, traps Tab/Shift+Tab within it while active, and restores focus to whatever was focused
// beforehand once the panel is removed.

const FOCUSABLE_SELECTOR =
	'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])';

function getFocusable(node: HTMLElement): HTMLElement[] {
	return Array.from(node.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR));
}

/**
 * Svelte action for a dialog panel. Pass `enabled: false` for placements that render inline
 * rather than as an overlay (for example Blade's staticPlacement), where trapping focus would
 * fight with the rest of the page.
 */
export function focusTrap(node: HTMLElement, enabled: boolean = true) {
	if (!enabled) return {};

	const previouslyFocused = document.activeElement as HTMLElement | null;

	const first = getFocusable(node)[0];
	if (first) {
		first.focus();
	} else {
		node.setAttribute('tabindex', '-1');
		node.focus();
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key !== 'Tab') return;
		const items = getFocusable(node);
		if (items.length === 0) return;
		const firstItem = items[0];
		const lastItem = items[items.length - 1];
		if (event.shiftKey && document.activeElement === firstItem) {
			event.preventDefault();
			lastItem.focus();
		} else if (!event.shiftKey && document.activeElement === lastItem) {
			event.preventDefault();
			firstItem.focus();
		}
	}

	node.addEventListener('keydown', handleKeydown);

	return {
		destroy() {
			node.removeEventListener('keydown', handleKeydown);
			previouslyFocused?.focus();
		}
	};
}
