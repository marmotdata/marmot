// Run with: node src/lib/metamodel/metamodel.selfcheck.mjs (Node strips the TypeScript types).
import assert from 'node:assert/strict';

import { resolveMessage } from './labels.ts';
import {
	draftFromValue,
	governedFields,
	governedPaths,
	omitPaths,
	parseIfMatchETag,
	readMetadataValue,
	sectionLabel,
	toPayload,
	typeLabel
} from './values.ts';

const field = (id, type, extra = {}) => ({
	id,
	type,
	core: true,
	required: false,
	storage: `metadata.example.${id}`,
	...extra
});

const retention = field('retention', 'integer', {
	required: true,
	validation: { minimum: 1, maximum: 3650 },
	presentation: { section: 'governance', order: 10 }
});
const classification = field('classification', 'enum', {
	values: ['public', 'internal'],
	presentation: { section: 'governance', order: 20 }
});
const steward = field('data_steward', 'string', { nullable: true, validation: { maxLength: 5 } });
const score = field('quality_score', 'number', {
	nullable: true,
	validation: { minimum: 0, maximum: 1 }
});
const pii = field('contains_pii', 'boolean', { nullable: true });
const review = field('next_review', 'date', { nullable: true });
const days = field('sampling_days', 'list', {
	itemType: 'integer',
	validation: { minimum: 1, maxItems: 2 }
});
const channels = field('delivery_channels', 'list', { itemType: 'enum', values: ['sftp', 'api'] });
const nativeName = { ...field('name', 'string', { required: true }), storage: 'marmot.name' };

// Ordering: native fields are not governed rows; required first, then section/order/id.
assert.deepEqual(
	governedFields([classification, nativeName, retention, steward]).map((f) => f.id),
	['retention', 'classification', 'data_steward']
);
const compliance = field('regulations', 'list', {
	itemType: 'string',
	presentation: { section: 'compliance', order: 10 }
});
assert.deepEqual(
	governedFields([retention, classification, compliance]).map((f) => f.id),
	['retention', 'classification', 'regulations'],
	'sections keep profile order, not alphabetical order'
);
assert.deepEqual(governedPaths([retention, nativeName]), [['example', 'retention']]);

// Free metadata: only governed leaves disappear.
const governed = [
	['example', 'retention'],
	['example', 'classification']
];
assert.deepEqual(omitPaths({ example: { retention: 1, classification: 'x' } }, governed), {});
assert.deepEqual(omitPaths({ example: { retention: 1, note: 'kept' }, source: 'erp' }, governed), {
	example: { note: 'kept' },
	source: 'erp'
});
assert.deepEqual(omitPaths({ example: {}, other: 1 }, governed), { example: {}, other: 1 });
assert.deepEqual(omitPaths({ example: 'scalar' }, governed), { example: 'scalar' });
const input = { example: { retention: 1, note: 'n' } };
omitPaths(input, governed);
assert.deepEqual(input, { example: { retention: 1, note: 'n' } }, 'input is not mutated');

// Reading by binding.
assert.equal(readMetadataValue({ example: { retention: 30 } }, 'metadata.example.retention'), 30);
assert.equal(readMetadataValue({ example: {} }, 'metadata.example.retention'), undefined);
assert.equal(readMetadataValue({ example: 3 }, 'metadata.example.retention'), undefined);
assert.equal(readMetadataValue({ example: { retention: 1 } }, 'marmot.name'), undefined);

// Draft to payload, one case per type.
assert.deepEqual(toPayload(retention, '30'), { ok: true, value: 30 });
assert.deepEqual(toPayload(retention, ' 30 '), { ok: true, value: 30 });
assert.deepEqual(toPayload(retention, '3.5'), { ok: false, code: 'type' });
assert.deepEqual(toPayload(retention, '0'), { ok: false, code: 'range' });
assert.deepEqual(toPayload(retention, '3651'), { ok: false, code: 'range' });
assert.deepEqual(toPayload(retention, ''), { ok: false, code: 'required' });
assert.deepEqual(toPayload(classification, ''), { ok: false, code: 'not_nullable' });
assert.deepEqual(toPayload(classification, 'nope'), { ok: false, code: 'enum' });
assert.deepEqual(toPayload(classification, 'internal'), { ok: true, value: 'internal' });
assert.deepEqual(toPayload(steward, ''), { ok: true, value: null });
assert.deepEqual(toPayload(steward, 'abcdef'), { ok: false, code: 'length' });
assert.deepEqual(toPayload(score, '0'), { ok: true, value: 0 });
assert.deepEqual(toPayload(score, '1.5'), { ok: false, code: 'range' });
assert.deepEqual(toPayload(pii, 'false'), { ok: true, value: false });
assert.deepEqual(toPayload(pii, ''), { ok: true, value: null });
assert.deepEqual(toPayload(review, '2027-01-15'), { ok: true, value: '2027-01-15' });
assert.deepEqual(toPayload(review, '15/01/2027'), { ok: false, code: 'date' });
assert.deepEqual(toPayload(review, '2027-13-40'), { ok: false, code: 'date' });
assert.deepEqual(toPayload(days, ['7', '30']), { ok: true, value: [7, 30] });
assert.deepEqual(toPayload(days, ['0']), { ok: false, code: 'range' });
assert.deepEqual(toPayload(days, ['1', '2', '3']), { ok: false, code: 'items' });
assert.deepEqual(toPayload(days, ['x']), { ok: false, code: 'type' });
assert.deepEqual(toPayload(days, []), { ok: true, value: [] });
assert.deepEqual(toPayload(channels, ['sftp']), { ok: true, value: ['sftp'] });
assert.deepEqual(toPayload(channels, ['ftp']), { ok: false, code: 'enum' });

// Stored value to draft and label.
assert.equal(draftFromValue(retention, 30), '30');
assert.equal(draftFromValue(pii, false), 'false');
assert.equal(draftFromValue(steward, undefined), '');
assert.deepEqual(draftFromValue(days, [7, 30]), ['7', '30']);
assert.deepEqual(draftFromValue(days, undefined), []);
assert.equal(typeLabel(days), 'list<integer>');
assert.equal(typeLabel(retention), 'integer');
const steward2 = field('data_steward', 'string', { presentation: { control: 'user' } });
assert.equal(typeLabel(steward2), 'user');
assert.equal(sectionLabel('data_quality'), 'Data quality');
assert.equal(sectionLabel('compliance'), 'Compliance');
assert.equal(sectionLabel(''), '');

// Labels: profile locale, profile default locale, Marmot catalogue, then nothing.
const messages = { es: { a: 'A-es' }, en: { a: 'A-en', b: 'B-en' } };
const native = (key) => (key === 'c' ? 'C-native' : undefined);
const ctx = { locale: 'es', defaultLocale: 'en', messages, native };
assert.equal(resolveMessage('a', ctx), 'A-es');
assert.equal(resolveMessage('b', ctx), 'B-en');
assert.equal(resolveMessage('c', ctx), 'C-native');
assert.equal(resolveMessage('d', ctx), undefined);
assert.equal(resolveMessage(undefined, ctx), undefined);
assert.equal(resolveMessage('a', { locale: 'es', defaultLocale: 'en' }), undefined);

// Versions travel as quoted entity tags.
assert.equal(parseIfMatchETag('"3"'), 3);
assert.equal(parseIfMatchETag('3'), null);
assert.equal(parseIfMatchETag('W/"3"'), null);

console.log('metamodel self-check ok');
