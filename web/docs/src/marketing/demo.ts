/**
 * Plays a scripted product demo: the real design-system components, driven step by step. A demo
 * only plays while it is on screen, pauses when it leaves or when the visitor presses Pause, and
 * starts over after a rest. Under reduced motion it runs straight to its finished state and stays
 * there, so nothing moves on its own.
 */

/** Thrown inside a script when the demo is torn down; the player swallows it. */
class Stopped extends Error {}

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
}

const reducedMotion = () =>
	typeof matchMedia !== 'undefined' && matchMedia('(prefers-reduced-motion: reduce)').matches;

export function play(node: HTMLElement, script: DemoScript): Player {
	let visible = false;
	let userPaused = false;
	let stopped = false;
	let resume: (() => void) | null = null;
	const still = reducedMotion();

	const running = () => visible && !userPaused && !stopped;
	const gate = () =>
		running() || stopped ? Promise.resolve() : new Promise<void>((r) => (resume = r));

	async function wait(ms: number) {
		if (stopped) throw new Stopped();
		if (still) return;
		let left = ms;
		while (left > 0) {
			await gate();
			if (stopped) throw new Stopped();
			const slice = Math.min(left, 100);
			await new Promise((r) => setTimeout(r, slice));
			if (running()) left -= slice;
		}
	}

	async function animate(ms: number, onFrame: (progress: number) => void) {
		if (stopped) throw new Stopped();
		if (still) return onFrame(1);
		let t = 0;
		await gate();
		let last = performance.now();
		while (t < ms) {
			const now = await new Promise<number>((r) => requestAnimationFrame(r));
			if (stopped) throw new Stopped();
			if (!running()) {
				await gate();
				last = performance.now();
				continue;
			}
			t += Math.min(now - last, 50);
			last = now;
			onFrame(Math.min(1, t / ms));
		}
	}

	const demo: Demo = { wait, animate, still };

	async function loop() {
		try {
			for (;;) {
				script.reset();
				await gate();
				await script.run(demo);
				if (still) return;
				await wait(script.rest ?? 4000);
			}
		} catch (e) {
			if (!(e instanceof Stopped)) throw e;
		}
	}

	const wake = () => {
		if (running() && resume) {
			const r = resume;
			resume = null;
			r();
		}
	};

	const io = new IntersectionObserver(
		([entry]) => {
			visible = !!entry?.isIntersecting;
			wake();
		},
		{ threshold: 0.4 }
	);
	io.observe(node);
	void loop();

	const player: Player = {
		get paused() {
			return userPaused;
		},
		toggle() {
			userPaused = !userPaused;
			player.onChange?.(userPaused);
			wake();
		},
		destroy() {
			stopped = true;
			io.disconnect();
			resume?.();
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
