<script lang="ts">
	import { onMount, type Snippet } from 'svelte';
	import { page } from '$app/stores';
	import { resolve } from '$app/paths';
	import IconifyIcon from '@iconify/svelte';
	import { m } from '$lib/paraglide/messages';
	import {
		SUBNAV_ASIDE,
		SUBNAV_CONTENT,
		SUBNAV_ITEM,
		SUBNAV_ITEM_OFF,
		SUBNAV_ITEM_ON,
		SUBNAV_TITLE
	} from '$lib/ui/classes';
	import { accountCapabilities, type AccountCapabilities } from '$lib/account/capabilities';

	let { children }: { children: Snippet } = $props();

	// The same shell as Settings, so personal and workspace settings read as one system with two owners.
	let capabilities = $state<AccountCapabilities>({ localAccount: false, totpAvailable: false });

	onMount(async () => {
		capabilities = await accountCapabilities();
	});

	let sections = $derived([
		{
			href: '/account' as const,
			label: m.account_profile(),
			icon: 'material-symbols:person-outline'
		},
		{
			href: '/account/subscriptions' as const,
			label: m.account_subscriptions(),
			icon: 'material-symbols:notifications-outline'
		},
		{
			href: '/account/api-keys' as const,
			label: m.account_api_keys(),
			icon: 'material-symbols:key-outline'
		},
		...(capabilities.localAccount
			? [
					{
						href: '/account/password' as const,
						label: m.profile_change_password(),
						icon: 'material-symbols:password'
					}
				]
			: []),
		...(capabilities.totpAvailable
			? [
					{
						href: '/account/totp' as const,
						label: m.totp_title(),
						icon: 'material-symbols:phonelink-lock-outline'
					}
				]
			: [])
	]);

	function active(href: string): boolean {
		const path = $page.url.pathname;
		return href === '/account' ? path === '/account' : path.startsWith(href);
	}
</script>

<div class="flex w-full flex-col lg:flex-row">
	<aside class={SUBNAV_ASIDE}>
		<p class={SUBNAV_TITLE}>{m.account_title()}</p>
		<nav
			aria-label={m.account_nav_aria()}
			class="flex gap-1 overflow-x-auto lg:flex-col lg:gap-0.5 lg:overflow-visible"
		>
			{#each sections as section (section.href)}
				<a
					href={resolve(section.href)}
					class="{SUBNAV_ITEM} {active(section.href) ? SUBNAV_ITEM_ON : SUBNAV_ITEM_OFF}"
					aria-current={active(section.href) ? 'page' : undefined}
				>
					<IconifyIcon icon={section.icon} class="h-4 w-4 flex-shrink-0 text-gray-400" />
					{section.label}
				</a>
			{/each}
		</nav>
	</aside>

	<div class={SUBNAV_CONTENT}>
		{@render children()}
	</div>
</div>
