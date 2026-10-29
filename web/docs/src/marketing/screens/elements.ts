/**
 * Defines every design-system element the product screens use. The landing page is prerendered,
 * and defining custom elements needs `customElements`, so the screens import only `lit` and data
 * and this runs in the browser before the first screen is drawn. Defining twice is a no-op, so the
 * other previews may import the same elements.
 */
let defined: Promise<void> | undefined;

export function defineScreenElements(): Promise<void> {
	defined ??= Promise.all([
		import('@marmotdata/icons/define.js'),
		import('@marmotdata/elements/app-shell.js'),
		import('@marmotdata/elements/sidebar-nav.js'),
		import('@marmotdata/elements/top-bar.js'),
		import('@marmotdata/elements/breadcrumbs.js'),
		import('@marmotdata/elements/logo.js'),
		import('@marmotdata/elements/command-palette.js'),
		import('@marmotdata/elements/audit-row.js'),
		import('@marmotdata/elements/button.js'),
		import('@marmotdata/elements/callout.js'),
		import('@marmotdata/elements/code-sample.js'),
		import('@marmotdata/elements/count.js'),
		import('@marmotdata/elements/data-table.js'),
		import('@marmotdata/elements/description-list.js'),
		import('@marmotdata/elements/details-blade.js'),
		import('@marmotdata/elements/disclosure.js'),
		import('@marmotdata/elements/empty-state.js'),
		import('@marmotdata/elements/facet-sidebar.js'),
		import('@marmotdata/elements/kbd.js'),
		import('@marmotdata/elements/lineage-canvas.js'),
		import('@marmotdata/elements/link.js'),
		import('@marmotdata/elements/list.js'),
		import('@marmotdata/elements/menu-button.js'),
		import('@marmotdata/elements/menu.js'),
		import('@marmotdata/elements/owner-chip.js'),
		import('@marmotdata/elements/page-header.js'),
		import('@marmotdata/elements/pagination.js'),
		import('@marmotdata/elements/popularity.js'),
		import('@marmotdata/elements/progress.js'),
		import('@marmotdata/elements/query-history.js'),
		import('@marmotdata/elements/results-grid.js'),
		import('@marmotdata/elements/run-strip.js'),
		import('@marmotdata/elements/schema-table.js'),
		import('@marmotdata/elements/search-bar.js'),
		import('@marmotdata/elements/search-field.js'),
		import('@marmotdata/elements/section.js'),
		import('@marmotdata/elements/segmented-control.js'),
		import('@marmotdata/elements/select.js'),
		import('@marmotdata/elements/split-pane.js'),
		import('@marmotdata/elements/sql-editor.js'),
		import('@marmotdata/elements/status-pill.js'),
		import('@marmotdata/elements/tabs.js'),
		import('@marmotdata/elements/tag.js'),
		import('@marmotdata/elements/text.js'),
		import('@marmotdata/elements/toast.js'),
		import('@marmotdata/elements/toolbar.js'),
		import('@marmotdata/elements/tree.js'),
		import('@marmotdata/elements/visually-hidden.js')
	]).then(() => undefined);
	return defined;
}
