/**
 * The one app shell every screen uses, so navigation is identical across the tour.
 * IA: Catalog / Work / Operate groups in a sidebar, Settings at the foot,
 * global search and notifications in the top bar.
 *
 * Embedded in the landing page rather than filling a window: the host sets `--screen-h`, and no
 * keyboard shortcut is registered on the window (`no-shortcut` turns off the shell's ⌘B), so the
 * screen never takes keys from the page around it.
 */
import { html, nothing, type TemplateResult } from 'lit';
import type {
	PaletteCommand,
	PaletteItem,
	PaletteResult,
	PaletteSearch
} from '@marmotdata/elements/classes/command-palette.js';
import { assets, glossary, products, teams } from './data/catalog';

export type NavId =
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
	| 'none';

export interface ShellOptions {
	current: NavId;
	/** Breadcrumb trail; the last item is the current page. */
	crumbs?: { label: string; href?: string }[];
	content: TemplateResult;
	blade?: TemplateResult;
	unread?: number;
	/** Full-bleed pages (Query, Lineage) manage their own padding. */
	bleed?: boolean;
}

const item = (
	id: NavId,
	current: NavId,
	icon: string,
	label: string,
	extra: { count?: number; attention?: number; attentionLabel?: string } = {}
) =>
	html`<mm-nav-item
		href="#${id}"
		icon=${icon}
		?current=${id === current}
		.count=${extra.count}
		.attention=${extra.attention}
		attention-label=${extra.attentionLabel ?? ''}
		>${label}</mm-nav-item
	>`;

const paletteCommands: PaletteCommand[] = [
	{ id: 'home', label: 'Home', icon: 'home', href: '#home' },
	{
		id: 'discover',
		label: 'Discover',
		icon: 'travel-explore',
		href: '#discover',
		keywords: ['search', 'browse', 'assets']
	},
	{
		id: 'glossary',
		label: 'Glossary',
		icon: 'menu-book',
		href: '#glossary',
		keywords: ['terms', 'definitions']
	},
	{ id: 'products', label: 'Data products', icon: 'package-2', href: '#products' },
	{ id: 'query', label: 'Query', icon: 'terminal', href: '#query', keywords: ['sql', 'editor'] },
	{ id: 'insights', label: 'Insights', icon: 'insights', href: '#insights' },
	{
		id: 'plugins',
		label: 'Plugins',
		icon: 'extension',
		href: '#plugins',
		keywords: ['pipelines', 'ingestion', 'runs']
	},
	{ id: 'audit', label: 'Query audit', icon: 'policy', href: '#audit' },
	{ id: 'rules', label: 'Asset rules', icon: 'rule', href: '#rules' },
	{
		id: 'settings',
		label: 'Settings',
		icon: 'settings',
		href: '#settings',
		group: 'Settings',
		keywords: ['members', 'teams', 'sso']
	},
	{
		id: 'api-keys',
		label: 'Account › API keys',
		icon: 'key',
		href: '#account',
		group: 'Account',
		keywords: ['tokens', 'credentials']
	},
	{
		id: 'notifications',
		label: 'Account › Notifications',
		icon: 'notifications',
		href: '#account',
		group: 'Account'
	}
];

const catalogResults: PaletteResult[] = [
	...assets.map((a): PaletteResult => ({
		id: a.mrn,
		kind: 'asset',
		label: a.name,
		mrn: a.mrn,
		provider: a.provider,
		href: '#asset'
	})),
	...glossary.map((g): PaletteResult => ({
		id: g.id,
		kind: 'glossary',
		label: g.name,
		description: g.definition,
		href: '#glossary'
	})),
	...products.map((d): PaletteResult => ({
		id: d.id,
		kind: 'data-product',
		label: d.name,
		description: d.description,
		href: '#products'
	})),
	...teams.map((t): PaletteResult => ({ id: t.id, kind: 'team', label: t.name, href: '#settings' }))
];

const paletteRecents: PaletteItem[] = catalogResults.filter((r) =>
	[
		'mrn://table/dbt/analytics.marts.orders_daily',
		'gmv',
		'mrn://table/snowflake/analytics.marts.customers_dim'
	].includes(r.id)
);

const paletteSearch: PaletteSearch = (query, signal) =>
	new Promise((resolve, reject) => {
		const words = query.toLowerCase().split(/\s+/).filter(Boolean);
		const t = setTimeout(
			() =>
				resolve(
					catalogResults
						.filter((r) =>
							words.every((w) => `${r.label} ${r.mrn ?? ''}`.toLowerCase().includes(w))
						)
						.slice(0, 8)
				),
			150
		);
		signal.addEventListener('abort', () => {
			clearTimeout(t);
			reject(new DOMException('Aborted', 'AbortError'));
		});
	});

// The mockups open the palette from a ⌘K listener on the window; here the top bar's search button
// is the only way in, so the landing page keeps its own keys.
const paletteIn = (el: Element | null) =>
	el?.closest('mm-app-shell')?.querySelector('mm-command-palette');

export function shell(o: ShellOptions): TemplateResult {
	const c = o.current;
	return html`<mm-app-shell style="height:var(--screen-h, 100vh)" no-shortcut ?bleed=${o.bleed}>
		<mm-sidebar-nav slot="sidebar" label="Main">
			<mm-logo slot="header" size="24"></mm-logo>
			<mm-nav-group label="Catalog">
				${item('home', c, 'home', 'Home')} ${item('discover', c, 'travel-explore', 'Discover')}
				${item('glossary', c, 'menu-book', 'Glossary')}
				${item('products', c, 'package-2', 'Data products')}
			</mm-nav-group>
			<mm-nav-group label="Work">
				${item('query', c, 'terminal', 'Query')} ${item('insights', c, 'insights', 'Insights')}
			</mm-nav-group>
			<mm-nav-group label="Operate">
				${item('plugins', c, 'extension', 'Plugins', { attention: 1, attentionLabel: '1 pipeline failed' })}
				${item('audit', c, 'policy', 'Query audit')} ${item('rules', c, 'rule', 'Asset rules')}
			</mm-nav-group>
			<div slot="footer">
				${item('settings', c, 'settings', 'Settings', { attention: 2, attentionLabel: '2 settings need attention' })}
			</div>
		</mm-sidebar-nav>
		<mm-top-bar
			slot="topbar"
			notifications
			.unread=${o.unread ?? 3}
			help-href="#docs"
			@mm-open-search=${(e: Event) => paletteIn(e.currentTarget as Element)?.show()}
		>
			${
				o.crumbs?.length
					? html`<mm-breadcrumbs slot="start">
							${o.crumbs.map((b, i) => html`<mm-breadcrumb href=${i < o.crumbs!.length - 1 ? (b.href ?? '#') : ''}>${b.label}</mm-breadcrumb>`)}
						</mm-breadcrumbs>`
					: nothing
			}
			<mm-avatar-menu
				slot="end"
				name="Jane Doe"
				email="jane.doe@shopwell.io"
				theme="system"
			></mm-avatar-menu>
		</mm-top-bar>
		<mm-command-palette
			search-href="#discover?q={query}"
			.commands=${paletteCommands}
			.recents=${paletteRecents}
			.search=${paletteSearch}
		></mm-command-palette>
		${o.content} ${o.blade ? html`<div slot="blade">${o.blade}</div>` : nothing}
	</mm-app-shell>`;
}
