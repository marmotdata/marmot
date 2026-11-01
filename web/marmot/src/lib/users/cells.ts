import { html, nothing, type TemplateResult } from 'lit';
import type { MenuSelectDetail } from '@marmotdata/elements/classes/menu.js';

export interface RowMenuItem {
	value: string;
	label: string;
	icon: string;
	danger?: boolean;
}

/**
 * The row's More menu for mm-data-table cells: a ghost icon button holding the occasional and destructive actions, with destructive items after a separator.
 */
export function rowMenu(
	label: string,
	items: RowMenuItem[],
	onSelect: (value: string) => void
): TemplateResult | typeof nothing {
	if (items.length === 0) return nothing;
	const safe = items.filter((i) => !i.danger);
	const danger = items.filter((i) => i.danger);
	const item = (i: RowMenuItem) =>
		html`<mm-menu-item value=${i.value} icon=${i.icon} ?danger=${!!i.danger}
			>${i.label}</mm-menu-item
		>`;
	return html`<mm-menu-button
		variant="ghost"
		size="sm"
		icon="more-horiz"
		icon-only
		label=${label}
		placement="bottom-end"
	>
		<mm-menu
			label=${label}
			@mm-select=${(e: CustomEvent<MenuSelectDetail>) => onSelect(e.detail.value)}
		>
			${safe.map(item)}
			${safe.length && danger.length ? html`<mm-menu-separator></mm-menu-separator>` : nothing}
			${danger.map(item)}
		</mm-menu>
	</mm-menu-button>`;
}
