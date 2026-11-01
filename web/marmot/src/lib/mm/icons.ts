/** Maps an Iconify Material Symbols name ("material-symbols:person-outline") to the design system's mm-icon name ("person"). */
export function mmIcon(name: string): string {
	return name
		.replace(/^material-symbols:/, '')
		.replace(/-outline(-rounded)?$/, '')
		.replace(/-rounded$/, '');
}
