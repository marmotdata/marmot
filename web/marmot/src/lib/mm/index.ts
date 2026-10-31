// The design-system elements most screens use, defined once for the whole app. Heavy elements (sql-editor, code-editor, lineage-canvas, results-grid, charts) are imported by the pages that use them so they stay out of the first load.
import '@marmotdata/icons/define.js';
import '@marmotdata/elements/app-shell.js';
import '@marmotdata/elements/sidebar-nav.js';
import '@marmotdata/elements/top-bar.js';
import '@marmotdata/elements/breadcrumbs.js';
import '@marmotdata/elements/logo.js';
import '@marmotdata/elements/banner.js';
import '@marmotdata/elements/command-palette.js';
import '@marmotdata/elements/button.js';
import '@marmotdata/elements/menu.js';
import '@marmotdata/elements/menu-button.js';
import '@marmotdata/elements/page-header.js';
import '@marmotdata/elements/panel.js';
import '@marmotdata/elements/section.js';
import '@marmotdata/elements/list.js';
import '@marmotdata/elements/text.js';
import '@marmotdata/elements/link.js';
import '@marmotdata/elements/tag.js';
import '@marmotdata/elements/status-pill.js';
import '@marmotdata/elements/empty-state.js';
import '@marmotdata/elements/callout.js';
import '@marmotdata/elements/avatar.js';
import '@marmotdata/elements/owner-chip.js';
import '@marmotdata/elements/tabs.js';
import '@marmotdata/elements/segmented-control.js';
import '@marmotdata/elements/dialog.js';
import '@marmotdata/elements/confirm-dialog.js';
import '@marmotdata/elements/details-blade.js';
import '@marmotdata/elements/description-list.js';
import '@marmotdata/elements/tooltip.js';
import '@marmotdata/elements/toast.js';
import '@marmotdata/elements/text-field.js';
import '@marmotdata/elements/textarea.js';
import '@marmotdata/elements/select.js';
import '@marmotdata/elements/checkbox.js';
import '@marmotdata/elements/switch.js';
import '@marmotdata/elements/form-field.js';
import '@marmotdata/elements/search-field.js';
import '@marmotdata/elements/pagination.js';
import '@marmotdata/elements/data-table.js';
import '@marmotdata/elements/skeleton.js';
import '@marmotdata/elements/loading.js';
import '@marmotdata/elements/copy-button.js';
import '@marmotdata/elements/kbd.js';
import '@marmotdata/elements/count.js';
import '@marmotdata/elements/readout.js';
import '@marmotdata/elements/disclosure.js';
import '@marmotdata/elements/popover.js';
import '@marmotdata/elements/topography.js';

export { setCrumbs, setBleed, chrome, type Crumb } from './chrome';
export { default as BladeSlot } from './BladeSlot.svelte';
export { mmIcon } from './icons';

// Svelte sets mm-button's `type` as a property, and @marmotdata/elements 0.1.1 neither reflects it nor checks the property when Enter in a field looks for the form's submit button, so Enter did nothing in multi-field forms. Mirror it to the attribute until a release with the design-system fix is installed.
{
	const proto = customElements.get('mm-button')?.prototype as HTMLElement | undefined;
	const desc = proto && Object.getOwnPropertyDescriptor(proto, 'type');
	if (proto && desc?.set && desc.get) {
		Object.defineProperty(proto, 'type', {
			...desc,
			set(this: HTMLElement, value: string) {
				desc.set!.call(this, value);
				if (value && value !== 'button') {
					if (this.getAttribute('type') !== value) this.setAttribute('type', value);
				} else if (this.hasAttribute('type')) this.removeAttribute('type');
			}
		});
	}
}
