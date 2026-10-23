// The class strings behind the shared visual language of the Plugins, Insights, Query and home pages, so every page reaches for the same surface, control and table instead of re-spelling them.

/** A white panel on the page background: the one surface every module sits on. */
export const PANEL =
	'rounded-xl border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800';

/** The primary action: one per view, top right of the page header or the panel it acts on. */
export const BTN_PRIMARY =
	'inline-flex items-center justify-center gap-2 rounded-lg bg-earthy-terracotta-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:bg-earthy-terracotta-500 focus:outline-none focus-visible:ring-2 focus-visible:ring-earthy-terracotta-500/40 disabled:cursor-not-allowed disabled:opacity-50';

/** The quieter action beside a primary one, and the default for row actions. */
export const BTN_SECONDARY =
	'inline-flex items-center justify-center gap-2 rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm font-medium text-gray-700 transition hover:border-gray-300 hover:bg-gray-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-earthy-terracotta-500/40 disabled:cursor-not-allowed disabled:opacity-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700';

/** A borderless action for toolbars and tight rows. */
export const BTN_GHOST =
	'inline-flex items-center justify-center gap-1.5 rounded-md px-2.5 py-1.5 text-sm font-medium text-gray-600 transition hover:bg-gray-100 hover:text-gray-900 focus:outline-none focus-visible:ring-2 focus-visible:ring-earthy-terracotta-500/40 disabled:cursor-not-allowed disabled:opacity-50 dark:text-gray-300 dark:hover:bg-gray-700 dark:hover:text-gray-100';

/** A destructive action. Red is reserved for this and for errors. */
export const BTN_DANGER =
	'inline-flex items-center justify-center gap-2 rounded-lg bg-red-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:bg-red-500 focus:outline-none focus-visible:ring-2 focus-visible:ring-red-500/40 disabled:cursor-not-allowed disabled:opacity-50';

/** Text inputs, selects and textareas. */
export const INPUT =
	'w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm text-gray-900 placeholder:text-gray-400 focus:border-earthy-terracotta-500 focus:outline-none focus:ring-2 focus:ring-earthy-terracotta-500/30 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100';

/** The search field above a list, with a leading icon inside a label. */
export const SEARCH_FIELD =
	'flex items-center gap-2 rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-sm focus-within:ring-2 focus-within:ring-earthy-terracotta-500/30 dark:border-gray-700 dark:bg-gray-800';

/** The small uppercase label over a field group, KPI or column. */
export const LABEL =
	'text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400';

/** Data tables: a quiet header, comfortable rows and a hover state, never a filled header band. */
export const TABLE = 'min-w-full text-sm';
export const TH =
	'px-4 py-2.5 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400';
export const TD = 'px-4 py-3 text-gray-700 dark:text-gray-300';
export const TR = 'transition-colors hover:bg-gray-50 dark:hover:bg-gray-700/40';
export const TBODY = 'divide-y divide-gray-100 dark:divide-gray-700/60';

/** A link inside running text or a module header. */
export const LINK =
	'font-medium text-earthy-terracotta-700 hover:underline dark:text-earthy-terracotta-400';

/** Filter pills above a list. The selected pill is inverted, like a segmented control. */
export const FILTER_PILL =
	'inline-flex items-center gap-1 rounded-full border px-3 py-1 text-xs font-medium transition';
export const FILTER_PILL_ON =
	'border-gray-900 bg-gray-900 text-white dark:border-gray-100 dark:bg-gray-100 dark:text-gray-900';
export const FILTER_PILL_OFF =
	'border-gray-200 bg-white text-gray-700 hover:border-gray-300 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200';

/** The left sub-navigation of a multi-section area such as Settings or Account: quieter than the content, with no panel of its own. */
export const SUBNAV_ASIDE =
	'flex-shrink-0 border-b border-gray-200 px-4 py-4 dark:border-gray-800 sm:px-6 lg:sticky lg:top-0 lg:h-[calc(100dvh-4rem)] lg:w-60 lg:overflow-y-auto lg:border-b-0 lg:border-r lg:px-3 lg:py-6';
export const SUBNAV_TITLE =
	'mb-3 hidden px-2.5 text-sm font-semibold text-gray-900 dark:text-gray-100 lg:block';
export const SUBNAV_ITEM =
	'flex items-center gap-2.5 rounded-lg px-2.5 py-1.5 text-sm transition-colors whitespace-nowrap';
export const SUBNAV_ITEM_ON =
	'bg-white font-medium text-gray-900 shadow-sm ring-1 ring-gray-200 dark:bg-gray-800 dark:text-gray-100 dark:ring-gray-700';
export const SUBNAV_ITEM_OFF =
	'text-gray-600 hover:bg-white/70 hover:text-gray-900 dark:text-gray-400 dark:hover:bg-gray-800/60 dark:hover:text-gray-100';
/** The content column beside a sub-navigation. */
export const SUBNAV_CONTENT = 'min-w-0 flex-1 px-4 py-6 sm:px-6 lg:px-8';
