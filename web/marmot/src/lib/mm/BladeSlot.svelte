<script lang="ts">
	import type { Snippet } from 'svelte';

	// Moves its content into the app shell's blade column, so a page can open a details blade on the end edge without the layout knowing about it. Blades opened by a person behave like the design system's overlay blade from the keyboard: focus moves into the blade, Escape closes it and focus returns to what opened it. `persistent` blades (the asset page's) are part of the page, so they leave focus alone.
	let {
		children,
		label = '',
		persistent = false
	}: { children: Snippet; label?: string; persistent?: boolean } = $props();

	/** The focused element, looking through open shadow roots. */
	function deepActive(): HTMLElement | null {
		let a = document.activeElement as HTMLElement | null;
		while (a?.shadowRoot?.activeElement) a = a.shadowRoot.activeElement as HTMLElement;
		return a && a !== document.body ? a : null;
	}

	function contains(node: HTMLElement, el: Element | null): boolean {
		for (
			let n: Node | null = el;
			n;
			n = (n as Element).assignedSlot ?? n.parentNode ?? (n as ShadowRoot).host ?? null
		) {
			if (n === node) return true;
		}
		return false;
	}

	/** True while a dialog, drawer or menu is open, so their own Escape handling wins. */
	function overlayOpen(): boolean {
		return !!document.querySelector(
			'mm-dialog[open], mm-confirm-dialog[open], mm-drawer[open], mm-command-palette[open], dialog[open]'
		);
	}

	function portal(node: HTMLElement) {
		const shell = document.querySelector('mm-app-shell');
		if (!shell) return;
		const returnTo = persistent ? null : deepActive();
		node.setAttribute('slot', 'blade');
		if (label) shell.setAttribute('blade-label', label);
		shell.appendChild(node);

		const close = () => {
			const blade = node.querySelector('mm-details-blade');
			const button = blade?.shadowRoot?.querySelector<HTMLElement>('[part~="close"]');
			button?.click();
		};

		const onKeydown = (e: KeyboardEvent) => {
			if (e.key !== 'Escape' || e.defaultPrevented || overlayOpen()) return;
			const active = deepActive();
			if (contains(node, active) || (returnTo && active === returnTo)) {
				e.preventDefault();
				close();
			}
		};
		// A press anywhere outside the blade closes it, like the drawer's backdrop; the opening click itself has already happened, and presses inside its menus and popovers belong to the blade
		const onPointerDown = (e: PointerEvent) => {
			if (e.button !== 0 || overlayOpen()) return;
			if (e.composedPath().includes(node)) return;
			close();
		};
		if (!persistent) {
			window.addEventListener('keydown', onKeydown);
			window.addEventListener('pointerdown', onPointerDown, true);
		}

		// Only when someone opened it from the page: a blade that is there on load must not take focus
		if (returnTo)
			requestAnimationFrame(() => {
				const blade = node.querySelector('mm-details-blade');
				const target =
					blade?.shadowRoot?.querySelector<HTMLElement>('[part~="close"]') ??
					node.querySelector<HTMLElement>('a[href], button, [tabindex]:not([tabindex="-1"])');
				target?.focus();
			});

		return {
			destroy() {
				window.removeEventListener('keydown', onKeydown);
				window.removeEventListener('pointerdown', onPointerDown, true);
				const active = deepActive();
				const hadFocus = !active || contains(node, active);
				node.remove();
				if (label) shell.removeAttribute('blade-label');
				if (hadFocus && returnTo?.isConnected) returnTo.focus();
			}
		};
	}
</script>

<div use:portal>
	{@render children()}
</div>
