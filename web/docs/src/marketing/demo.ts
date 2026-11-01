/**
 * Plays a scripted product demo: the real design-system components, driven step by step. A demo
 * only plays while it is on screen, pauses when it leaves or when the visitor presses Pause, and
 * starts over after a rest. Under reduced motion it runs straight to its finished state and stays
 * there, so nothing moves on its own.
 */

/** Thrown inside a script when the demo is torn down; the player swallows it. */
class Stopped extends Error {}
/** Thrown inside a script when the visitor seeks; the player starts the run again from the top. */
class Restart extends Error {}

export interface Demo {
	/** Waits `ms` of playing time: paused time does not count, and reduced motion skips it. */
	wait(ms: number): Promise<void>;
	/** Calls `onFrame` with progress from 0 to 1 over `ms` of playing time, once a frame. */
	animate(ms: number, onFrame: (progress: number) => void): Promise<void>;
	/** True under reduced motion: scripts set their end state without stepping through it. */
	readonly still: boolean;
}

export interface DemoScript {
	/** Puts every component back to where the script begins. */
	reset(): void;
	/** The steps, awaiting `demo.wait` between them. */
	run(demo: Demo): Promise<void>;
	/** Rest between the end and the next run. */
	rest?: number;
}

export interface Player {
	/** Whether the visitor has paused it (being off screen is not a pause). */
	readonly paused: boolean;
	toggle(): void;
	destroy(): void;
	/** Called with the paused state whenever it changes, for the Pause button. */
	onChange?: (paused: boolean) => void;
	/** Playing time into the current run, in ms, rest included. */
	readonly elapsed: number;
	/** True under reduced motion, where the demo shows finished states rather than playing. */
	readonly still: boolean;
	/** Plays the run again from `ms` in, skipping straight past what comes before. Under reduced
	 *  motion it stops there, so each point can still be seen as a still. */
	seek(ms: number): void;
}

const reducedMotion = () =>
	typeof matchMedia !== 'undefined' && matchMedia('(prefers-reduced-motion: reduce)').matches;

export function play(node: HTMLElement, script: DemoScript): Player {
	let visible = false;
	let userPaused = false;
	let stopped = false;
	let resume: (() => void) | null = null;
	let interrupt: (() => void) | null = null;
	const still = reducedMotion();
	// Each run belongs to a generation; seeking starts a new one, and the old run's next wait throws.
	let gen = 0;
	let played = 0;
	let skipTo = 0;
	let hold = false;

	const running = () => visible && !userPaused && !stopped;
	const gate = () =>
		running() || stopped ? Promise.resolve() : new Promise<void>((r) => (resume = r));
	const check = (g: number) => {
		if (stopped) throw new Stopped();
		if (g !== gen) throw new Restart();
	};
	// Waits for the next seek or teardown, for a still demo that has nothing left to play.
	const halt = async (g: number) => {
		await new Promise<void>((r) => (interrupt = r));
		check(g);
	};

	const demoFor = (g: number): Demo => ({
		still,
		async wait(ms: number) {
			check(g);
			// Seeking runs straight through everything before the point it seeks to.
			if (played + ms <= skipTo) {
				played += ms;
				return;
			}
			if (played < skipTo) {
				ms -= skipTo - played;
				played = skipTo;
			}
			if (still) {
				if (hold) return halt(g);
				// A still demo jumps to its end; count the time so its captions land there too.
				played += ms;
				return;
			}
			let left = ms;
			while (left > 0) {
				await gate();
				check(g);
				const slice = Math.min(left, 100);
				await new Promise((r) => setTimeout(r, slice));
				check(g);
				if (running()) {
					left -= slice;
					played += slice;
				}
			}
		},
		async animate(ms: number, onFrame: (progress: number) => void) {
			check(g);
			if (still || played + ms <= skipTo) {
				played += ms;
				return onFrame(1);
			}
			let t = Math.max(0, skipTo - played);
			played += t;
			await gate();
			let last = performance.now();
			while (t < ms) {
				const now = await new Promise<number>((r) => requestAnimationFrame(r));
				check(g);
				if (!running()) {
					await gate();
					last = performance.now();
					continue;
				}
				const step = Math.min(now - last, 50, ms - t);
				t += step;
				played += step;
				last = now;
				onFrame(Math.min(1, t / ms));
			}
		}
	});

	async function loop() {
		for (;;) {
			const g = gen;
			played = 0;
			try {
				script.reset();
				if (skipTo === 0) await gate();
				check(g);
				const demo = demoFor(g);
				await script.run(demo);
				if (still) await halt(g);
				await demo.wait(script.rest ?? 4000);
				skipTo = 0;
			} catch (e) {
				if (e instanceof Restart) continue;
				if (e instanceof Stopped) return;
				throw e;
			}
		}
	}

	const wake = () => {
		if (running() && resume) {
			const r = resume;
			resume = null;
			r();
		}
	};
	// Lets a waiting run see that it has been replaced or torn down.
	const release = () => {
		const r = resume;
		const i = interrupt;
		resume = interrupt = null;
		r?.();
		i?.();
	};

	// On screen means 40% of it is showing, or, for something taller than the screen, that it
	// fills 40% of the screen; otherwise a tall demo could never count as on screen.
	const io = new IntersectionObserver(
		([entry]) => {
			const screen = entry?.rootBounds?.height ?? innerHeight;
			visible = !!entry?.isIntersecting && (entry.intersectionRatio >= 0.4 || entry.intersectionRect.height >= screen * 0.4);
			wake();
		},
		{ threshold: Array.from({ length: 11 }, (_, i) => i / 10) }
	);
	io.observe(node);
	void loop();

	const player: Player = {
		get paused() {
			return userPaused;
		},
		get elapsed() {
			return played;
		},
		still,
		toggle() {
			userPaused = !userPaused;
			player.onChange?.(userPaused);
			wake();
		},
		seek(ms: number) {
			gen++;
			skipTo = ms;
			hold = still;
			release();
		},
		destroy() {
			stopped = true;
			io.disconnect();
			release();
		}
	};
	return player;
}

/** Types `text` into `set` a few characters at a time, as a person would. */
export async function type(demo: Demo, text: string, set: (value: string) => void, perChar = 45) {
	if (demo.still) return set(text);
	for (let i = 1; i <= text.length; i++) {
		set(text.slice(0, i));
		await demo.wait(perChar);
	}
}
