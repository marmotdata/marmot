/**
 * Checks messages/en.json against the m.<key>() calls in src.
 * Missing keys always fail; unused keys only warn unless --strict is passed.
 */

import { readFileSync, readdirSync } from 'fs';
import { join, dirname, relative, sep } from 'path';
import { fileURLToPath } from 'url';

const rootDir = join(dirname(fileURLToPath(import.meta.url)), '..');
const strict = process.argv.includes('--strict');

const catalogue = JSON.parse(readFileSync(join(rootDir, 'messages', 'en.json'), 'utf8'));
const definedKeys = new Set(Object.keys(catalogue).filter((key) => key !== '$schema'));

const callPattern = /\bm\.([A-Za-z_$][A-Za-z0-9_$]*)\s*\(/g;

// A message put in a lookup table is referenced without being called, so bare references count too
const referencePattern = /\bm\.([A-Za-z_$][A-Za-z0-9_$]*)/g;

// Plenty of callbacks name their parameter m, which makes bare references in such a file ambiguous rather than evidence that a key must exist
const shadowedPattern = /\(\s*m\s*[),:]|\bm\s*=>|\bconst\s+m\b|\blet\s+m\b/;

const calledKeys = new Map();
const referencedKeys = new Set();
const bareKeys = new Map();

function scan(dir) {
	for (const entry of readdirSync(dir, { withFileTypes: true })) {
		const full = join(dir, entry.name);
		if (entry.isDirectory()) {
			// The compiled Paraglide output defines the messages, it does not use them.
			if (relative(rootDir, full) === join('src', 'lib', 'paraglide')) continue;
			scan(full);
			continue;
		}
		if (!entry.name.endsWith('.svelte') && !entry.name.endsWith('.ts')) continue;
		const content = readFileSync(full, 'utf8');
		for (const match of content.matchAll(callPattern)) {
			const key = match[1];
			if (!calledKeys.has(key)) calledKeys.set(key, []);
			calledKeys.get(key).push(relative(rootDir, full).split(sep).join('/'));
		}
		const shadowed = shadowedPattern.test(content);
		for (const match of content.matchAll(referencePattern)) {
			referencedKeys.add(match[1]);
			if (!shadowed && !calledKeys.has(match[1])) {
				if (!bareKeys.has(match[1])) bareKeys.set(match[1], []);
				bareKeys.get(match[1]).push(relative(rootDir, full).split(sep).join('/'));
			}
		}
	}
}

scan(join(rootDir, 'src'));

const usedKeys = new Map([...calledKeys, ...bareKeys]);
const undefinedKeys = [...usedKeys.keys()].filter((key) => !definedKeys.has(key)).sort();
const unusedKeys = [...definedKeys].filter((key) => !referencedKeys.has(key)).sort();

if (undefinedKeys.length > 0) {
	console.error(`Used but not defined in messages/en.json (${undefinedKeys.length}):`);
	for (const key of undefinedKeys) {
		const files = [...new Set(usedKeys.get(key))];
		console.error(`  ${key}  (${files.slice(0, 3).join(', ')}${files.length > 3 ? ', …' : ''})`);
	}
}

if (unusedKeys.length > 0) {
	console.warn(`Defined but never used (${unusedKeys.length}):`);
	for (const key of unusedKeys) {
		console.warn(`  ${key}`);
	}
}

if (undefinedKeys.length === 0 && unusedKeys.length === 0) {
	console.log(`i18n check passed: ${definedKeys.size} keys defined, all in use.`);
} else if (undefinedKeys.length === 0) {
	console.log(`i18n check passed: ${definedKeys.size} keys defined, ${unusedKeys.length} unused.`);
}

if (undefinedKeys.length > 0 || (strict && unusedKeys.length > 0)) {
	process.exit(1);
}
