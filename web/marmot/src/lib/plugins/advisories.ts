import type { PluginAdvisory } from './types';

// Advisory severities, worst first, so a single "most urgent" one can stand in for the rest in a
// badge or a banner. The GHSA vocabulary is low, moderate, high and critical; "medium" is kept as
// an alias of "moderate" for anything still using the older word. A severity outside that list
// ranks below "low" rather than being treated as nothing to worry about.
const SEVERITY_RANK: Record<string, number> = {
	critical: 4,
	high: 3,
	moderate: 2,
	medium: 2,
	low: 1
};

export function severityRank(severity: string): number {
	return SEVERITY_RANK[severity.toLowerCase()] ?? 0;
}

/** The most urgent of a plugin's advisories, for a badge that can only show one. */
export function worstAdvisory(advisories: PluginAdvisory[]): PluginAdvisory | undefined {
	return [...advisories].sort((a, b) => severityRank(b.severity) - severityRank(a.severity))[0];
}
