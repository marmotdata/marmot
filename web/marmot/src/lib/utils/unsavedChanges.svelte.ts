import { beforeNavigate, goto } from '$app/navigation';

export interface UnsavedChangesGuard {
	/** Whether the discard-changes confirmation should be shown. */
	readonly showConfirm: boolean;
	/** Discard the unsaved changes and continue to the navigation that was blocked. */
	confirmDiscard: () => void;
	/** Keep editing; stay on the current page. */
	cancelDiscard: () => void;
}

/**
 * Guards in-app navigation and tab close/reload while `isDirty()` is true. In-app navigation is
 * cancelled and re-tried through `goto()` once the caller confirms via a ConfirmModal (bound to
 * `showConfirm`); closing the tab or reloading falls back to the browser's own beforeunload prompt,
 * since that is the only dialog the platform allows there.
 */
export function useUnsavedChangesGuard(isDirty: () => boolean): UnsavedChangesGuard {
	let showConfirm = $state(false);
	let pendingUrl: URL | null = null;
	let bypassNextNavigation = false;

	beforeNavigate((navigation) => {
		if (bypassNextNavigation) {
			bypassNextNavigation = false;
			return;
		}
		if (!isDirty() || navigation.type === 'leave') return;
		navigation.cancel();
		pendingUrl = navigation.to?.url ?? null;
		showConfirm = true;
	});

	$effect(() => {
		if (!isDirty()) return;
		const handleBeforeUnload = (event: BeforeUnloadEvent) => {
			event.preventDefault();
			event.returnValue = '';
		};
		window.addEventListener('beforeunload', handleBeforeUnload);
		return () => window.removeEventListener('beforeunload', handleBeforeUnload);
	});

	function confirmDiscard() {
		showConfirm = false;
		const url = pendingUrl;
		pendingUrl = null;
		if (url) {
			bypassNextNavigation = true;
			// url is the destination SvelteKit itself already resolved for the navigation we blocked, not a hand-built path
			// eslint-disable-next-line svelte/no-navigation-without-resolve
			goto(url);
		}
	}

	function cancelDiscard() {
		showConfirm = false;
		pendingUrl = null;
	}

	return {
		get showConfirm() {
			return showConfirm;
		},
		confirmDiscard,
		cancelDiscard
	};
}
