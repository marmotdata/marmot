<script lang="ts">
	import '$lib/icon-bundle';
	import '$lib/mm';
	import { onMount, type Snippet } from 'svelte';
	import '../app.css';
	import { page } from '$app/state';
	import { auth, isAnonymousMode } from '$lib/stores/auth';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { browser } from '$app/environment';
	import { m } from '$lib/paraglide/messages';
	import { settingsAttention, refreshSettingsAttention } from '$lib/settings/attention';
	import { myAccess, loadMyAccess, holdsAtRoot } from '$lib/iam/myAccess';
	import { visibleItems } from '$lib/settings/sections';
	import { locale } from '$lib/i18n';
	import { userProfile } from '$lib/stores/user';
	import { theme } from '$lib/stores/theme';
	import { notifications, unreadCount } from '$lib/stores/notifications';
	import { encryptionConfigured, allowUnencrypted } from '$lib/stores/encryption';
	import { tablePreviewEnabled } from '$lib/stores/features';
	import { fetchApi } from '$lib/api';
	import { recordView, viewedItem } from '$lib/home/api';
	import { chrome, mmIcon, type Crumb } from '$lib/mm';
	import Banner from '$lib/components/Banner.svelte';
	import ToastContainer from '$components/ui/ToastContainer.svelte';
	import type {
		MmCommandPalette,
		PaletteCommand,
		PaletteResult,
		PaletteSearch
	} from '@marmotdata/elements/command-palette.js';

	let { children }: { children: Snippet } = $props();

	interface BannerConfig {
		enabled: boolean;
		dismissible: boolean;
		variant: 'info' | 'warning' | 'error' | 'success';
		message: string;
		id: string;
	}

	let bannerConfig = $state<BannerConfig | null>(null);
	let checkingAnonymousMode = $state(true);
	let manualNavigation = false;
	let palette = $state<MmCommandPalette>();

	const path = $derived(page.url.pathname);
	// Invitation links are opened by people who cannot sign in yet, so they get the bare sign-in layout too
	const bare = $derived(path.startsWith('/login') || path.startsWith('/invite/'));

	onMount(() => {
		init();
		return () => notifications.stopPolling();
	});

	async function init() {
		checkingAnonymousMode = true;
		await auth.checkAnonymousMode();
		checkingAnonymousMode = false;

		if ($auth) {
			userProfile.load();
			notifications.startPolling();
		}

		try {
			const response = await fetch('/api/v1/ui/config');
			if (response.ok) {
				const data = await response.json();
				bannerConfig = data.banner;
				encryptionConfigured.set(data.encryption_configured ?? true);
				allowUnencrypted.set(data.allow_unencrypted ?? false);
				tablePreviewEnabled.set(data.table_preview_enabled ?? false);
			}
		} catch (error) {
			console.error('Failed to fetch UI config:', error);
		}
	}

	$effect(() => {
		if (browser && $auth) loadMyAccess();
	});

	const canQueryData = $derived(browser && !!$auth && auth.hasPermission('assets', 'query'));
	const canViewQueryLog = $derived(browser && !!$auth && holdsAtRoot($myAccess, 'access:view'));
	const settingsItems = $derived(browser && $auth ? visibleItems($myAccess) : []);
	const canSeeSettings = $derived(settingsItems.length > 0);

	// Re-checked on navigation, at most every few minutes, so the dot clears without a reload once things are fixed
	$effect(() => {
		if (canSeeSettings && $myAccess.checked && path) refreshSettingsAttention($myAccess);
	});

	// Opening an asset, term, product or saved query puts it on the viewer's Jump back in list on Home
	$effect(() => {
		if (browser && $auth) recordView(viewedItem(page.route.id, page.params, page.url.searchParams));
	});

	$effect(() => {
		if (!browser || checkingAnonymousMode || manualNavigation) return;
		const isOAuthPending = page.url.searchParams.has('oauth_pending');
		if ($auth && path.startsWith('/login') && !isOAuthPending) {
			goto(resolve('/'));
		} else if (!$auth && !bare && !$isAnonymousMode) {
			goto(resolve('/login'));
		}
	});

	function withManualNav(fn: () => void) {
		manualNavigation = true;
		fn();
		setTimeout(() => {
			manualNavigation = false;
		}, 100);
	}

	function signOut() {
		withManualNav(() => {
			auth.clearToken();
			userProfile.clear();
			window.location.href = '/';
		});
	}

	type NavId =
		| 'home'
		| 'discover'
		| 'glossary'
		| 'products'
		| 'query'
		| 'insights'
		| 'plugins'
		| 'audit'
		| 'rules'
		| 'settings'
		| 'account'
		| 'notifications'
		| 'none';

	const current = $derived.by((): NavId => {
		if (path === '/') return 'home';
		const top = path.split('/')[1];
		switch (top) {
			case 'discover':
			case 'search':
			case 'assets':
				return 'discover';
			case 'glossary':
				return 'glossary';
			case 'products':
				return 'products';
			case 'query':
				return 'query';
			case 'insights':
			case 'metrics':
				return 'insights';
			case 'runs':
			case 'pipelines':
			case 'plugins':
				return 'plugins';
			case 'query-audit':
				return 'audit';
			case 'asset-rules':
				return 'rules';
			case 'settings':
			case 'teams':
			case 'users':
			case 'roles':
			case 'service-accounts':
			case 'admin':
				return 'settings';
			case 'account':
			case 'profile':
				return 'account';
			case 'notifications':
				return 'notifications';
			default:
				return 'none';
		}
	});

	const navLabel: Record<NavId, () => string> = {
		home: m.nav_home,
		discover: m.nav_discover,
		glossary: m.nav_glossary,
		products: m.nav_data_products,
		query: m.nav_query,
		insights: m.nav_insights,
		plugins: m.nav_runs,
		audit: m.nav_query_audit,
		rules: m.nav_asset_rules,
		settings: m.nav_settings,
		account: m.nav_account,
		notifications: m.pagetitle_notifications,
		none: () => ''
	};

	// A page that sets its own crumbs wins; otherwise the crumb is the section the route belongs to
	const crumbs = $derived.by((): Crumb[] => {
		if ($chrome.path === path && $chrome.crumbs) return $chrome.crumbs;
		const label = navLabel[current]();
		return label ? [{ label }] : [];
	});
	const bleed = $derived(
		($chrome.path === path && $chrome.bleed) ||
			path.startsWith('/query') ||
			path.startsWith('/glossary')
	);

	const title = $derived.by(() => {
		void $locale;
		const names = crumbs.map((c) => c.label).reverse();
		return names.length && current !== 'home' ? `${names.join(' · ')} · Marmot` : 'Marmot';
	});

	const settingsLabel = $derived(
		canSeeSettings && $settingsAttention > 0
			? m.settings_attention_dot({ count: $settingsAttention })
			: ''
	);

	// The palette offers every page the caller may open, from the same definitions as the sidebar and the Settings sub-navigation, so it never offers a page they cannot see
	const commands = $derived.by((): PaletteCommand[] => {
		void $locale;
		const list: PaletteCommand[] = [
			{ id: 'home', label: m.nav_home(), icon: 'home', href: '/' },
			{
				id: 'discover',
				label: m.nav_discover(),
				icon: 'travel-explore',
				href: '/discover',
				keywords: ['search', 'browse', 'assets']
			},
			{
				id: 'glossary',
				label: m.nav_glossary(),
				icon: 'menu-book',
				href: '/glossary',
				keywords: ['terms', 'definitions']
			},
			{ id: 'products', label: m.nav_data_products(), icon: 'package-2', href: '/products' }
		];
		if (canQueryData)
			list.push({
				id: 'query',
				label: m.nav_query(),
				icon: 'terminal',
				href: '/query',
				keywords: ['sql', 'editor']
			});
		list.push(
			{ id: 'insights', label: m.nav_insights(), icon: 'insights', href: '/insights' },
			{
				id: 'plugins',
				label: m.nav_runs(),
				icon: 'extension',
				href: '/runs',
				keywords: ['pipelines', 'ingestion', 'runs']
			}
		);
		if (canViewQueryLog)
			list.push({ id: 'audit', label: m.nav_query_audit(), icon: 'policy', href: '/query-audit' });
		list.push({ id: 'rules', label: m.nav_asset_rules(), icon: 'rule', href: '/asset-rules' });
		if (canSeeSettings) {
			list.push({
				id: 'settings',
				label: m.nav_settings(),
				icon: 'settings',
				href: '/settings',
				group: m.nav_settings()
			});
			for (const item of settingsItems)
				list.push({
					id: `settings-${item.id}`,
					label: `${m.nav_settings()} › ${item.label()}`,
					icon: mmIcon(item.icon),
					href: item.href,
					group: m.nav_settings(),
					keywords: item.keywords().split(/\s+/)
				});
		}
		if ($auth)
			list.push(
				{
					id: 'account',
					label: m.nav_account(),
					icon: 'account-circle',
					href: '/account',
					group: m.nav_account(),
					keywords: m.account_profile_keywords().split(/\s+/)
				},
				{
					id: 'account-notifications',
					label: `${m.nav_account()} › ${m.pagetitle_notifications()}`,
					icon: 'notifications',
					href: '/account/notifications',
					group: m.nav_account()
				},
				{
					id: 'api-keys',
					label: `${m.nav_account()} › ${m.account_api_keys()}`,
					icon: 'key',
					href: '/account/api-keys',
					group: m.nav_account(),
					keywords: m.account_api_keys_keywords().split(/\s+/)
				}
			);
		return list;
	});

	interface SearchResult {
		type: 'asset' | 'glossary' | 'team' | 'data_product';
		id: string;
		name: string;
		description?: string;
		metadata?: Record<string, unknown>;
		url: string;
	}

	const kindOf: Record<SearchResult['type'], PaletteResult['kind']> = {
		asset: 'asset',
		glossary: 'glossary',
		team: 'team',
		data_product: 'data-product'
	};

	const search: PaletteSearch = async (query, signal) => {
		const response = await fetchApi(`/search?q=${encodeURIComponent(query)}&limit=8`, { signal });
		if (!response.ok) return [];
		const data: { results?: SearchResult[] } = await response.json();
		return (data.results ?? []).map((r): PaletteResult => {
			const meta = r.metadata ?? {};
			const providers = meta.providers;
			const provider =
				typeof meta.provider === 'string'
					? meta.provider
					: Array.isArray(providers) && typeof providers[0] === 'string'
						? providers[0]
						: undefined;
			return {
				id: `${r.type}:${r.id}`,
				kind: kindOf[r.type] ?? 'asset',
				label: r.name,
				mrn: typeof meta.mrn === 'string' ? meta.mrn : undefined,
				provider: provider?.toLowerCase(),
				description: r.type === 'asset' ? undefined : r.description,
				href: r.url
			};
		});
	};

	// The palette navigates with location by default; routing through SvelteKit keeps the app loaded
	function onPaletteSelect(e: CustomEvent<{ item: { href?: string; run?: () => void } }>) {
		const href = e.detail?.item?.href;
		if (!href || e.detail.item.run) return;
		e.preventDefault();
		goto(resolve(href as '/'));
	}

	function onSearchAll(e: CustomEvent<{ href?: string; query?: string }>) {
		e.preventDefault();
		const q = e.detail?.query ?? '';
		goto(resolve(`/discover?q=${encodeURIComponent(q)}`));
	}

	function onAvatarSelect(e: CustomEvent<{ value: string }>) {
		const v = e.detail?.value;
		if (v === 'account') goto(resolve('/account'));
		else if (v === 'language') goto(resolve('/account'));
		else if (v === 'sign-out') signOut();
	}

	function onThemeChange(e: CustomEvent<{ theme: 'light' | 'dark' | 'system' }>) {
		e.preventDefault();
		const next = e.detail.theme === 'system' ? 'auto' : e.detail.theme;
		theme.set(next);
		fetchApi('/users/preferences', {
			method: 'PUT',
			body: JSON.stringify({ preferences: { theme: next } })
		}).catch((error) => console.error('Failed to update theme preference:', error));
	}

	const themeChoice = $derived($theme === 'auto' ? 'system' : $theme);
	const profileName = $derived($userProfile?.name || $userProfile?.username || '');
</script>

<svelte:head>
	<title>{title}</title>
</svelte:head>

<!-- Keyed on the locale so a language change re-renders the shell in place instead of reloading the document -->
{#key $locale}
	{#if bare}
		{@render children()}
	{:else}
		<mm-app-shell {bleed}>
			{#if bannerConfig}
				<div slot="banner">
					<Banner
						enabled={bannerConfig.enabled}
						dismissible={bannerConfig.dismissible}
						variant={bannerConfig.variant}
						message={bannerConfig.message}
						id={bannerConfig.id}
					/>
				</div>
			{/if}
			<mm-sidebar-nav slot="sidebar" label={m.nav_main_label()}>
				<a slot="header" href={resolve('/')} aria-label="Marmot">
					<mm-logo size={24}></mm-logo>
				</a>
				<mm-nav-group label={m.nav_group_catalog()}>
					<mm-nav-item href={resolve('/')} icon="home" current={current === 'home'}
						>{m.nav_home()}</mm-nav-item
					>
					<mm-nav-item
						href={resolve('/discover')}
						icon="travel-explore"
						current={current === 'discover'}>{m.nav_discover()}</mm-nav-item
					>
					<mm-nav-item href={resolve('/glossary')} icon="menu-book" current={current === 'glossary'}
						>{m.nav_glossary()}</mm-nav-item
					>
					<mm-nav-item href={resolve('/products')} icon="package-2" current={current === 'products'}
						>{m.nav_data_products()}</mm-nav-item
					>
				</mm-nav-group>
				<mm-nav-group label={m.nav_group_work()}>
					{#if canQueryData}
						<mm-nav-item href={resolve('/query')} icon="terminal" current={current === 'query'}
							>{m.nav_query()}</mm-nav-item
						>
					{/if}
					<mm-nav-item href={resolve('/insights')} icon="insights" current={current === 'insights'}
						>{m.nav_insights()}</mm-nav-item
					>
				</mm-nav-group>
				<mm-nav-group label={m.nav_group_operate()}>
					<mm-nav-item href={resolve('/runs')} icon="extension" current={current === 'plugins'}
						>{m.nav_runs()}</mm-nav-item
					>
					{#if canViewQueryLog}
						<mm-nav-item href={resolve('/query-audit')} icon="policy" current={current === 'audit'}
							>{m.nav_query_audit()}</mm-nav-item
						>
					{/if}
					<mm-nav-item href={resolve('/asset-rules')} icon="rule" current={current === 'rules'}
						>{m.nav_asset_rules()}</mm-nav-item
					>
				</mm-nav-group>
				{#if canSeeSettings}
					<div slot="footer">
						<mm-nav-item
							href={resolve('/settings')}
							icon="settings"
							current={current === 'settings'}
							attention={$settingsAttention > 0 ? $settingsAttention : undefined}
							attention-label={settingsLabel}>{m.nav_settings()}</mm-nav-item
						>
					</div>
				{/if}
			</mm-sidebar-nav>
			<mm-top-bar
				slot="topbar"
				placeholder={m.nav_search_everything()}
				notifications={!!$auth}
				unread={$auth ? $unreadCount : 0}
				help-href="https://marmotdata.io/docs"
				onmm-open-search={() => palette?.show()}
				onmm-open-notifications={() => goto(resolve('/notifications'))}
			>
				{#if crumbs.length}
					<mm-breadcrumbs slot="start">
						{#each crumbs as crumb, i (i)}
							<mm-breadcrumb href={i < crumbs.length - 1 ? (crumb.href ?? '') : ''}
								>{crumb.label}</mm-breadcrumb
							>
						{/each}
					</mm-breadcrumbs>
				{/if}
				{#if $auth}
					<mm-avatar-menu
						slot="end"
						name={profileName}
						email={$userProfile?.email ?? ''}
						theme={themeChoice}
						onmm-select={onAvatarSelect}
						onmm-theme-change={onThemeChange}
					></mm-avatar-menu>
				{:else}
					<mm-button slot="end" variant="secondary" size="sm" icon="login" href={resolve('/login')}
						>{m.nav_login()}</mm-button
					>
				{/if}
			</mm-top-bar>
			<mm-command-palette
				bind:this={palette}
				global-shortcut
				search-href="/discover?q={'{query}'}"
				{commands}
				{search}
				onmm-select={onPaletteSelect}
				onmm-search-all={onSearchAll}
			></mm-command-palette>
			{@render children()}
		</mm-app-shell>
	{/if}
{/key}

<ToastContainer />
