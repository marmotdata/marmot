<script lang="ts">
	import '@marmotdata/elements/sparkline.js';

	export interface SparkPoint {
		label: string;
		value: number;
	}

	// Kept as a thin wrapper over mm-sparkline so existing callers keep their props; height is accepted but the sparkline stays word-sized
	let {
		points,
		tone = 'neutral',
		label = ''
	}: {
		points: SparkPoint[];
		tone?: 'neutral' | 'critical' | 'calm';
		height?: number;
		label?: string;
	} = $props();
</script>

<mm-sparkline
	values={points.map((p) => p.value)}
	{label}
	tone={tone === 'calm' ? 'info' : 'primary'}
	style={tone === 'critical' ? '--mm-chart-key: var(--mm-danger)' : undefined}
></mm-sparkline>
