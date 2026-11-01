// The cells for the core plugins table, kept out of the component so its script stays about
// wiring props to the table rather than markup. Row actions call back to `onAction`; the page owns
// the API calls, toasts and reload, so these functions only build what each cell shows.
import { html, nothing, type TemplateResult } from 'lit';
import type { DataTableColumn } from '@marmotdata/elements/classes/data-table.js';
import { m } from '$lib/paraglide/messages';
import { worstAdvisory } from '$lib/plugins/advisories';
import type { PluginVersionStatus } from '$lib/plugins/types';

export type PluginRowAction = 'latest' | 'choose' | 'pin' | 'unpin' | 'rollback' | 'reset';

const originLabel = (p: PluginVersionStatus) =>
	({
		release: m.plugin_versions_origin_release(),
		manual: m.plugin_versions_origin_manual(),
		auto: m.plugin_versions_origin_auto(),
		local: m.plugin_versions_origin_local()
	})[p.origin];

const nameCell = (p: PluginVersionStatus) =>
	html`<mm-provider-logo provider=${p.icon || p.id} size="16" decorative></mm-provider-logo
		>&nbsp;&nbsp;${p.name}`;

const versionCell = (p: PluginVersionStatus) => html`
	<div class="mm-stack" style="--mm-gap: var(--space-1)">
		<mm-text variant="mono-dense"
			>${p.origin === 'local' ? m.plugin_versions_local_version() : p.version}</mm-text
		>
		${
			p.error
				? html`<mm-text variant="caption" tone="danger"
						>${m.plugin_versions_switch_error({ error: p.error })}</mm-text
					>`
				: nothing
		}
	</div>
`;

const originCell = (p: PluginVersionStatus) => html`
	<span class="mm-cluster" style="--mm-gap: var(--space-1); flex-wrap: nowrap">
		<mm-tag>${originLabel(p)}</mm-tag>
		${p.pinned ? html`<mm-tag>${m.plugin_versions_pinned()}</mm-tag>` : nothing}
	</span>
`;

const statusCell = (p: PluginVersionStatus) => {
	const badges: TemplateResult[] = [];
	if (p.advisories.length > 0) {
		const worst = worstAdvisory(p.advisories);
		if (worst) {
			badges.push(html`
				<div class="mm-stack" style="--mm-gap: var(--space-1)">
					<mm-status-pill status="danger">${m.plugin_versions_security_update()}</mm-status-pill>
					<mm-text variant="caption" tone="secondary">
						${m.plugin_versions_advisory_summary({
							summary: worst.summary,
							id: worst.id,
							severity: worst.severity,
							fixed: worst.fixed
						})}
						${
							p.advisories.length > 1
								? ` ${m.plugin_versions_advisory_more({ count: p.advisories.length - 1 })}`
								: ''
						}
						${
							worst.url
								? html`&nbsp;<a
											class="mm-link"
											href=${worst.url}
											target="_blank"
											rel="external noopener noreferrer"
											>${m.plugin_versions_advisory_link()}</a
										>`
								: nothing
						}
					</mm-text>
				</div>
			`);
		}
	}
	if (p.update_available && p.latest) {
		badges.push(
			html`<mm-status-pill status="neutral"
				>${m.plugin_versions_update_to({ version: p.latest })}</mm-status-pill
			>`
		);
	}
	if (badges.length === 0) return nothing;
	return html`<div class="mm-stack" style="--mm-gap: var(--space-2)">${badges}</div>`;
};

function actionsCell(
	p: PluginVersionStatus,
	canManage: boolean,
	disabled: boolean,
	busyId: string | null,
	onAction: (plugin: PluginVersionStatus, action: PluginRowAction) => void
) {
	if (p.origin === 'local')
		return html`<mm-text variant="caption" tone="tertiary"
			>${m.plugin_versions_local_override_hint()}</mm-text
		>`;
	if (!canManage) return nothing;
	// Loading only shows on the row actually changing; disabled covers every row, so a second
	// change can never be issued, for this plugin or any other, while one is already in flight.
	const busy = busyId === p.id;
	return html`
		<span
			class="mm-cluster"
			style="--mm-gap: var(--space-1); justify-content: flex-end; flex-wrap: nowrap"
		>
			${
				p.update_available && p.latest
					? html`<mm-button
							size="sm"
							variant="ghost"
							icon="arrow-upward-alt"
							?loading=${busy}
							?disabled=${disabled}
							@click=${() => onAction(p, 'latest')}
							>${m.plugin_versions_action_update_latest()}</mm-button
						>`
					: nothing
			}
			<mm-menu-button
				size="sm"
				variant="ghost"
				icon="more-horiz"
				icon-only
				?disabled=${disabled}
				label=${m.plugin_versions_more_actions_for({ name: p.name })}
			>
				<mm-menu
					label=${m.plugin_versions_actions_for({ name: p.name })}
					@mm-select=${(e: CustomEvent<{ value: string }>) =>
						onAction(p, e.detail.value as PluginRowAction)}
				>
					${
						p.versions.length > 0
							? html`<mm-menu-item icon="list" value="choose"
									>${m.plugin_versions_action_choose_version()}</mm-menu-item
								>`
							: nothing
					}
					<mm-menu-item icon="push-pin" value=${p.pinned ? 'unpin' : 'pin'}>
						${p.pinned ? m.plugin_versions_action_unpin() : m.plugin_versions_action_pin()}
					</mm-menu-item>
					${
						p.previous_version || p.origin === 'manual' || p.origin === 'auto'
							? html`<mm-menu-separator></mm-menu-separator>`
							: nothing
					}
					${
						p.previous_version
							? html`<mm-menu-item icon="history" value="rollback"
									>${m.plugin_versions_action_rollback({ version: p.previous_version })}</mm-menu-item
								>`
							: nothing
					}
					${
						p.origin === 'manual' || p.origin === 'auto'
							? html`<mm-menu-item icon="restart-alt" value="reset"
									>${m.plugin_versions_action_reset()}</mm-menu-item
								>`
							: nothing
					}
				</mm-menu>
			</mm-menu-button>
		</span>
	`;
}

/** The core plugins table's columns, bound to one row's actions and the busy state of a change in flight. */
export function pluginVersionsColumns(
	canManage: boolean,
	disabled: boolean,
	busyId: string | null,
	onAction: (plugin: PluginVersionStatus, action: PluginRowAction) => void
): DataTableColumn<PluginVersionStatus>[] {
	return [
		{
			key: 'name',
			header: m.plugin_versions_col_plugin(),
			minWidth: 200,
			sortable: true,
			render: nameCell
		},
		{
			key: 'version',
			header: m.plugin_versions_col_version(),
			minWidth: 160,
			render: versionCell
		},
		{
			key: 'origin',
			header: m.plugin_versions_col_origin(),
			minWidth: 160,
			render: originCell
		},
		{
			key: 'status',
			header: m.common_status(),
			minWidth: 260,
			render: statusCell
		},
		{
			key: 'actions',
			header: m.common_actions(),
			minWidth: 200,
			align: 'end',
			render: (p) => actionsCell(p, canManage, disabled, busyId, onAction)
		}
	];
}
