// Token-based class strings for markup that cannot use an mm-* element directly (a native button inside a form, a table that is not an mm-data-table), so it still matches the design system in both themes without dark: variants.

/** A panel on the page background: surface, one hairline and the panel radius, never a shadow. */
export const PANEL = 'rounded-md border border-mm-rule bg-mm-surface';

const BTN_BASE =
	'inline-flex h-[var(--mm-control)] items-center justify-center gap-space-2 whitespace-nowrap rounded-md border px-[14px] text-button no-underline transition-colors duration-[var(--mm-dur-press)] disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50';

/** The primary action: one per view region, burnt sienna, as mm-button variant="primary". */
export const BTN_PRIMARY = `${BTN_BASE} border-transparent bg-mm-primary-fill text-mm-on-primary hover:bg-mm-primary-hover`;

/** The quieter action beside a primary one, as mm-button variant="secondary". */
export const BTN_SECONDARY = `${BTN_BASE} border-mm-control-border bg-mm-surface text-mm-text hover:border-mm-text`;

/** A borderless action for toolbars and tight rows, as mm-button variant="ghost". */
export const BTN_GHOST =
	'inline-flex h-[var(--control-h-sm)] items-center justify-center gap-space-2 whitespace-nowrap rounded-md border border-transparent bg-transparent px-[10px] text-button text-mm-text-2 transition-colors duration-[var(--mm-dur-press)] hover:bg-mm-sunken hover:text-mm-text disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50';

/** A destructive action in crimson; the label always says what it destroys, as mm-button variant="danger". */
export const BTN_DANGER = `${BTN_BASE} border-transparent bg-mm-danger-fill text-mm-on-primary hover:brightness-90`;

/** Text inputs, selects and textareas, matching mm-text-field. */
export const INPUT =
	'w-full rounded-sm border border-mm-control-border bg-mm-surface px-[10px] py-[6px] text-dense text-mm-text placeholder:text-mm-text-3 focus:border-mm-text focus:outline focus:outline-2 focus:outline-offset-2 focus:outline-mm-text';

/** The search field above a list, with a leading icon inside a label, matching mm-search-field. */
export const SEARCH_FIELD =
	'flex h-[var(--mm-control)] items-center gap-space-2 rounded-sm border border-mm-control-border bg-mm-surface px-[10px] text-dense text-mm-text focus-within:border-mm-text focus-within:outline focus-within:outline-2 focus-within:outline-offset-2 focus-within:outline-mm-text';

/** A field or column label: sentence case, never uppercase. */
export const LABEL = 'text-label text-mm-text-2';

/** Data tables: a sunken header band, hairline rows and a neutral hover. */
export const TABLE = 'min-w-full text-dense';
export const TH = 'bg-mm-sunken px-space-4 py-space-2 text-left text-label text-mm-text-2';
export const TD = 'px-space-4 py-space-2 text-mm-text';
export const TR = 'transition-colors hover:bg-[var(--mm-selected)]';
export const TBODY = 'divide-y divide-mm-rule';

/** A link inside running text: burnt sienna, underlined. Links in lists and rows use ink instead. */
export const LINK = 'text-mm-primary underline underline-offset-2 hover:text-mm-primary-hover';

/** Filter pills above a short fixed list. The selected pill is inverted ink, never the accent. */
export const FILTER_PILL =
	'inline-flex h-7 items-center gap-1.5 rounded-full border px-3 text-[13px] font-semibold transition-colors';
export const FILTER_PILL_ON = 'border-mm-text bg-mm-text text-mm-on-inverse';
export const FILTER_PILL_OFF =
	'border-mm-rule bg-mm-surface text-mm-text hover:border-mm-control-border';

/** The sub-navigation of a multi-section area such as Settings or Account: quieter than the content, with no panel of its own. */
export const SUBNAV_ASIDE =
	'flex-shrink-0 border-b border-mm-rule py-space-4 lg:sticky lg:top-0 lg:w-60 lg:self-start lg:border-b-0 lg:border-r lg:pr-space-4';
export const SUBNAV_TITLE = 'mb-space-3 hidden px-2.5 text-panel-title text-mm-text lg:block';
export const SUBNAV_ITEM =
	'flex items-center gap-2.5 whitespace-nowrap rounded-md px-2.5 py-1.5 text-dense transition-colors';
export const SUBNAV_ITEM_ON = 'bg-[var(--mm-selected)] font-semibold text-mm-text';
export const SUBNAV_ITEM_OFF = 'text-mm-text-2 hover:bg-mm-sunken hover:text-mm-text';
/** The content column beside a sub-navigation. */
export const SUBNAV_CONTENT = 'min-w-0 flex-1 lg:pl-space-8';
