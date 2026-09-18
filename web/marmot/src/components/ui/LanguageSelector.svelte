<script lang="ts">
	import Icon from '@iconify/svelte';
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';
	import { locales } from '$lib/paraglide/runtime';
	import { changeLocale, locale, type Locale } from '$lib/i18n';
	import { auth } from '$lib/stores/auth';
	import { toasts } from '$lib/stores/toast';

	type Props = { variant?: 'menu' | 'dropdown' };
	let { variant = 'dropdown' }: Props = $props();
	let saving = $state(false);
	let open = $state(false);
	let root = $state<HTMLDivElement>();
	let inMenu = $derived(variant === 'menu');
	let currentLocale = $derived($locale);
	let languageLabel = $derived.by(() => {
		void currentLocale;
		return m.profile_language_label();
	});

	function displayName(value: string): string {
		try {
			const name = new Intl.DisplayNames([value], { type: 'language' }).of(value);
			if (!name) return value;
			return name.charAt(0).toLocaleUpperCase(value) + name.slice(1);
		} catch {
			return value;
		}
	}

	function optionClass(value: Locale): string {
		return value === currentLocale
			? 'text-earthy-terracotta-700 dark:text-earthy-terracotta-700 font-medium'
			: 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700';
	}

	async function selectLocale(next: Locale) {
		open = false;
		if (next === currentLocale || saving) return;
		changeLocale(next);

		if (!auth.isAuthenticated()) return;
		saving = true;
		try {
			const response = await fetchApi('/users/preferences', {
				method: 'PUT',
				body: JSON.stringify({ preferences: { language: next } })
			});
			if (!response.ok) throw new Error(String(response.status));
		} catch (error) {
			console.error('Failed to update language preference:', error);
			toasts.error(m.profile_language_save_error());
		} finally {
			saving = false;
		}
	}

	function toggleList(event: MouseEvent) {
		event.stopPropagation();
		open = !open;
	}

	function handleWindowClick(event: MouseEvent) {
		if (!root?.contains(event.target as Node)) open = false;
	}

	function handleWindowKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') open = false;
	}
</script>

<svelte:window onclick={handleWindowClick} onkeydown={handleWindowKeydown} />

<div
	role="group"
	class={inMenu
		? 'border-t border-gray-200/80 dark:border-gray-700/70 px-3 py-2'
		: 'relative inline-flex'}
	bind:this={root}
	onpointerdown={(event) => {
		if (inMenu) event.stopPropagation();
	}}
>
	{#if inMenu}
		<p
			class="pb-1.5 text-[11px] font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400"
		>
			{languageLabel}
		</p>
	{/if}
	<div class="relative {inMenu ? 'block' : 'inline-flex'}">
		<button
			type="button"
			aria-haspopup="listbox"
			aria-expanded={open}
			aria-label={languageLabel}
			disabled={locales.length === 1 || saving}
			onclick={toggleList}
			class="inline-flex items-center gap-2 rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-900 px-3 py-2 text-left text-sm text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500 focus:border-transparent disabled:opacity-60 {inMenu
				? 'w-full justify-between'
				: ''}"
		>
			<span class="min-w-0 truncate">{displayName(currentLocale)}</span>
			<Icon
				icon="material-symbols:keyboard-arrow-down"
				class="h-4 w-4 shrink-0 text-gray-500 dark:text-gray-400 transition-transform {open
					? 'rotate-180'
					: ''}"
			/>
		</button>
		{#if open}
			<div
				class="mt-1 max-h-60 overflow-y-auto overscroll-contain rounded-md border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 shadow-lg {inMenu
					? 'relative w-full'
					: 'absolute z-50 min-w-full left-1/2 w-max -translate-x-1/2'}"
				role="listbox"
				aria-label={languageLabel}
			>
				{#each locales as value (value)}
					<button
						type="button"
						role="option"
						aria-selected={value === currentLocale}
						onclick={(event) => {
							event.stopPropagation();
							selectLocale(value);
						}}
						class="flex w-full items-center justify-between gap-3 px-3 py-2 text-left text-sm transition-colors {optionClass(
							value
						)}"
					>
						<span>{displayName(value)}</span>
						{#if value === currentLocale}
							<span class="h-1.5 w-1.5 rounded-full bg-earthy-terracotta-700" aria-hidden="true"
							></span>
						{/if}
					</button>
				{/each}
			</div>
		{/if}
	</div>
</div>
