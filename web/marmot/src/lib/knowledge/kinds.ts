import { m } from '#lib/paraglide/messages.js';

/** A kind of knowledge entry Marmot offers a starting point for. An entry is an ordinary asset whose type is the kind's, with a documentation page as its body, so nothing here is registered with the server and any other type still works through the API. */
export interface KnowledgeKind {
	/** The key used in URLs and filters. */
	id: string;
	/** The asset type stored on the entry. */
	type: string;
	icon: string;
	label: () => string;
	description: () => string;
	/** The Markdown the entry's first page starts with. It is seeded once as the author's own content, so it is not translated. */
	template: string;
}

/** The provider every entry written in Marmot carries, since an asset needs one and the entry lives nowhere else. */
export const KNOWLEDGE_PROVIDER = 'Marmot';

export const knowledgeKinds: KnowledgeKind[] = [
	{
		id: 'policy',
		type: 'Policy',
		icon: 'policy',
		label: m.knowledge_kind_policy,
		description: m.knowledge_kind_policy_description,
		template: `## What this policy says

State the rule in one or two sentences.

## Who it applies to

## What to do

-

## Exceptions and who approves them

## Who to ask
`
	},
	{
		id: 'decision',
		type: 'Decision',
		icon: 'flag',
		label: m.knowledge_kind_decision,
		description: m.knowledge_kind_decision_description,
		template: `## Decision

What was decided, in one sentence.

## Date and who decided

## Why

The problem, and the reasons this option won.

## Options that were turned down

-

## What changes as a result
`
	},
	{
		id: 'runbook',
		type: 'Runbook',
		icon: 'checklist',
		label: m.knowledge_kind_runbook,
		description: m.knowledge_kind_runbook_description,
		template: `## When to use this

## Before you start

-

## Steps

1.

## How to tell it worked

## If it goes wrong
`
	},
	{
		id: 'brand-voice',
		type: 'Brand Voice',
		icon: 'article',
		label: m.knowledge_kind_brand_voice,
		description: m.knowledge_kind_brand_voice_description,
		template: `## How we sound

Three or four words that describe the voice, each with a sentence on what it means in practice.

## Words we use

-

## Words we avoid

-

## Examples

A short passage written the way we want, and the same passage written the way we do not.

## How this changes by audience
`
	},
	{
		id: 'writing-style',
		type: 'Writing Style',
		icon: 'edit-note',
		label: m.knowledge_kind_writing_style,
		description: m.knowledge_kind_writing_style_description,
		template: `## Spelling and grammar

Which English, and the rules people most often get wrong.

## Formatting

Headings, lists, numbers, dates and capitalisation.

## Rules

-

## Examples

Before and after, for the rules that need showing.
`
	},
	{
		id: 'design-system',
		type: 'Design System',
		icon: 'category',
		label: m.knowledge_kind_design_system,
		description: m.knowledge_kind_design_system_description,
		template: `## Where it lives

Links to the design files, the component library and the code.

## Principles

-

## Colour, type and spacing

The tokens people need most, or a link to where they are kept.

## Components

Which to use for what, and which are retired.

## Who to ask
`
	}
];

/** The kind an asset type belongs to, or undefined for a type that is not a knowledge entry. */
export function knowledgeKindOf(type: string | null | undefined): KnowledgeKind | undefined {
	const t = (type ?? '').toLowerCase();
	return t ? knowledgeKinds.find((k) => k.type.toLowerCase() === t) : undefined;
}

/** The kind with the given key, as read from a URL. */
export function knowledgeKindById(id: string | null | undefined): KnowledgeKind | undefined {
	return id ? knowledgeKinds.find((k) => k.id === id) : undefined;
}
