const SKIPPED_KEY = 'marmot.home.skipped';

function readSkipped(): string[] {
	try {
		const saved = JSON.parse(localStorage.getItem(SKIPPED_KEY) ?? '[]');
		return Array.isArray(saved) ? saved : [];
	} catch {
		return [];
	}
}

// Optional setup steps the viewer chose to skip; only a per-browser convenience, so a blocked read just means nothing is skipped
function createSkippedStore() {
	let steps = $state<string[]>(typeof window === 'undefined' ? [] : readSkipped());

	function persist() {
		try {
			localStorage.setItem(SKIPPED_KEY, JSON.stringify(steps));
		} catch {
			// Remembering a skipped step is a convenience only
		}
	}

	return {
		get steps() {
			return steps;
		},
		toggle(step: string) {
			steps = steps.includes(step) ? steps.filter((s) => s !== step) : [...steps, step];
			persist();
		}
	};
}

export const skippedSteps = createSkippedStore();

// A remembered on/off choice about the setup guide; a blocked read just means the default
function createFlagStore(key: string) {
	let on = $state(false);
	try {
		on = typeof window !== 'undefined' && localStorage.getItem(key) === '1';
	} catch {
		// Fall back to off
	}

	return {
		get value() {
			return on;
		},
		set(value: boolean) {
			on = value;
			try {
				localStorage.setItem(key, value ? '1' : '0');
			} catch {
				// Remembering the choice is a convenience only
			}
		}
	};
}

// Hidden for now: the Setup pill in the homepage header brings the guide back
export const setupHidden = createFlagStore('marmot.home.setup-hidden');

// Removed for good: neither the guide nor the Setup pill shows again in this browser
export const setupRemoved = createFlagStore('marmot.home.setup-removed');
