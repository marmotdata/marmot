/**
 * Settings › Access › Service accounts › svc-finance-agent: what the account behind revenue-agent
 * may do. The mockup's service account page lists roles and API keys; here its roles are grants on
 * the catalog or on one asset, so the demo can grant asset.dataViewer on orders_daily and show it
 * arrive.
 */
import { html, nothing, type TemplateResult } from 'lit';
import type { DataTableColumn } from '@marmotdata/elements/classes/data-table.js';
import { shell } from './shell';
import { owner } from './data/catalog';
import {
	financeGrants,
	financeKeys,
	newGrant,
	serviceAccounts,
	type ApiKey,
	type Grant
} from './data/admin';
import { longStamp, rel } from './data/asset';

/* ------------------------------------------------------------ Layout */

type Section =
	'overview' | 'members' | 'teams' | 'service-accounts' | 'roles' | 'authentication' | 'system';
const attentionCount: Partial<Record<Section, { n: number; label: string }>> = {
	'service-accounts': { n: 2, label: '2 API keys need attention' },
	authentication: { n: 1, label: '1 authentication setting needs attention' }
};

const navItem = (id: Section, icon: string, label: string, current: Section) =>
	html`<mm-list-item href="#settings/${id}" ?selected=${id === current}>
		<mm-icon slot="start" name=${icon}></mm-icon>${label}
		${attentionCount[id] ? html`<mm-count slot="end" .value=${attentionCount[id]!.n} label=${attentionCount[id]!.label}></mm-count>` : nothing}
	</mm-list-item>`;

const settingsNav = (current: Section) =>
	html`<nav
		aria-label="Settings"
		class="mm-stack"
		style="--mm-gap: var(--space-5); position: sticky; top: 0"
	>
		<mm-search-field
			label="Search settings"
			hide-label
			placeholder="Search settings"
			shortcut="/"
		></mm-search-field>
		<mm-list dense>${navItem('overview', 'tune', 'Overview', current)}</mm-list>
		<div class="mm-stack" style="--mm-gap: var(--space-1)">
			<p class="mm-label" style="margin: 0; padding-inline: var(--space-4)" id="nav-access">
				Access
			</p>
			<mm-list dense aria-labelledby="nav-access">
				${navItem('members', 'person', 'Members', current)}
				${navItem('teams', 'groups', 'Teams', current)}
				${navItem('service-accounts', 'smart-toy', 'Service accounts', current)}
				${navItem('roles', 'admin-panel-settings', 'Roles', current)}
			</mm-list>
		</div>
		<div class="mm-stack" style="--mm-gap: var(--space-1)">
			<p class="mm-label" style="margin: 0; padding-inline: var(--space-4)" id="nav-security">
				Security
			</p>
			<mm-list dense aria-labelledby="nav-security"
				>${navItem('authentication', 'shield', 'Authentication', current)}</mm-list
			>
		</div>
		<div class="mm-stack" style="--mm-gap: var(--space-1)">
			<p class="mm-label" style="margin: 0; padding-inline: var(--space-4)" id="nav-system">
				System
			</p>
			<mm-list dense aria-labelledby="nav-system"
				>${navItem('system', 'dns', 'System', current)}</mm-list
			>
		</div>
	</nav>`;

const settings = (
	current: Section,
	body: TemplateResult,
	o: { extra?: TemplateResult; crumbs: { label: string; href?: string }[] }
) =>
	shell({
		current: 'settings',
		crumbs: [{ label: 'Settings', href: '#settings' }, ...o.crumbs],
		content: html`<div class="mm-split mm-split--sidebar">
				${settingsNav(current)}
				<div class="mm-stack" style="min-width: 0">${body}</div>
			</div>
			${o.extra ?? nothing}`
	});

/* ------------------------------------------------------------ Grants */

// Sized to fit beside the settings nav at the landing page's 1280px. Dates are relative to the
// screens' fixed "now", not the visitor's clock, which the data table's date type would use.
const grantColumns: DataTableColumn<Grant>[] = [
	{ key: 'role', header: 'Role', type: 'mono', width: 190 },
	{
		key: 'on',
		header: 'On',
		minWidth: 180,
		render: (g) =>
			g.provider
				? html`<span
						style="display: inline-flex; align-items: center; gap: var(--space-2); white-space: nowrap"
						><mm-provider-logo provider=${g.provider} size="16" decorative></mm-provider-logo
						>${g.on}</span
					>`
				: html`<span class="mm-muted">${g.on}</span>`
	},
	{ key: 'by', header: 'Granted by', width: 140 },
	{ key: 'at', header: 'Granted', width: 130, value: (g) => rel(g.at) }
];

/* ------------------------------------------------------------ API keys */

const keyColumns: DataTableColumn<ApiKey>[] = [
	{ key: 'name', header: 'Name', width: 220 },
	{ key: 'prefix', header: 'Key', type: 'mono', width: 160 },
	{
		key: 'created',
		header: 'Created',
		width: 130,
		value: (k) => longStamp(k.created).replace(/ \d{4},.*$/, '')
	},
	{ key: 'lastUsed', header: 'Last used', width: 130, value: (k) => rel(k.lastUsed) },
	{ key: 'state', header: 'Status', width: 170, minWidth: 160, render: () => 'Active' },
	{
		key: 'actions',
		header: 'Actions',
		width: 180,
		minWidth: 180,
		align: 'end',
		render: (k) =>
			html`<mm-button size="sm" variant="ghost" icon="delete" label="Revoke key ${k.name}"
				>Revoke</mm-button
			>`
	}
];

/* ------------------------------------------------------------ Screen */

export interface AccessState {
	/** Whether asset.dataViewer on orders_daily has been granted to svc-finance-agent. */
	granted: boolean;
}

const account = serviceAccounts.find((a) => a.id === 'svc-finance-agent')!;

export const accessScreen = (s: AccessState): TemplateResult => {
	const grants = s.granted ? [newGrant, ...financeGrants] : financeGrants;
	return settings(
		'service-accounts',
		html`<mm-page-header
				heading=${account.name}
				icon="smart-toy"
				caption="Service account · Last used 1 minute ago"
				description=${account.description}
			>
				<span slot="owners" class="mm-cluster"
					><span class="mm-caption">Used by</span
					><mm-owner-chip
						name=${account.agent}
						kind="agent"
						service-account=${account.name}
					></mm-owner-chip
				></span>
				<mm-menu-button
					slot="actions"
					variant="ghost"
					icon="more-horiz"
					icon-only
					label="More actions for ${account.name}"
				>
					<mm-menu
						><mm-menu-item icon="delete" danger
							>Delete service account ${account.name}</mm-menu-item
						></mm-menu
					>
				</mm-menu-button>
				<mm-button slot="actions" variant="primary" icon="add">Grant access</mm-button>
			</mm-page-header>
			<mm-description-list layout="inline-grid" columns="4">
				<mm-description term="Grants">${grants.length}</mm-description>
				<mm-description term="Created"
					>${longStamp(account.created).replace(/,.*$/, '')}</mm-description
				>
				<mm-description term="Created by">${owner('aoife').name}</mm-description>
				<mm-description term="Calls in the last 7 days">1,204</mm-description>
			</mm-description-list>
			<mm-section
				heading="Access"
				description="Roles on the whole catalog or on one asset. Someone with several roles gets every permission in them."
			>
				<mm-data-table
					label="Access of ${account.name}"
					row-key="id"
					selectable
					.columns=${grantColumns}
					.rows=${grants}
					.selected=${s.granted ? [newGrant.id] : []}
				></mm-data-table>
			</mm-section>
			<mm-section heading="API keys" description="A key is shown only once, when it is created.">
				<mm-button slot="actions" icon="key">Create API key</mm-button>
				<mm-data-table
					label="API keys for ${account.name}"
					.columns=${keyColumns}
					.rows=${financeKeys}
				></mm-data-table>
			</mm-section>`,
		{
			crumbs: [
				{ label: 'Service accounts', href: '#settings/service-accounts' },
				{ label: account.name }
			],
			// Persistent, so it stays for as long as the demo shows the granted state.
			extra: s.granted
				? html`<mm-toaster label="Notifications"
						><mm-toast
							tone="success"
							duration="0"
							message="Granted asset.dataViewer on orders_daily to ${account.name}."
						></mm-toast
					></mm-toaster>`
				: undefined
		}
	);
};
