---
title: Knowledge
description: Write down your company's policies, decisions, runbooks, incidents, guides, facts, brand voice, writing styles and design systems in Marmot, link them to the assets they govern and let people and agents find the ones that apply.
---

# Knowledge

Knowledge entries hold what your company has written down about how it works: policies, decisions, runbooks, incident write-ups, guides, references to documents kept elsewhere, facts, brand voice, writing styles and design systems. Each entry links to the tables, services, repositories or dashboards it applies to, so anyone looking at an asset sees the rules that govern it. Agents read the same entries through the [MCP server](MCP/index.md) before they write code, docs or copy for you.

An entry is an asset in the catalog like any other. It has owners and tags, shows up in search and has a write-up in Markdown. What makes it knowledge is its kind, its status and what it's linked to.

## Kinds of entry

Each kind asks for a few details of its own and starts the write-up with an outline:

| Kind | What it holds | Details it asks for |
| --- | --- | --- |
| Policy | A rule people are expected to follow | Who it applies to, who approved it and when, who to ask for an exception |
| Decision | What was decided, when and by whom, and what it replaced | Decision date (required once accepted or rejected), who decided |
| Runbook | The steps for a task people do again and again | The alerts or symptoms that call for it, who to escalate to |
| Incident | What went wrong, why and what changed after | When it started (required once published) and was resolved, severity, a link to the incident record, follow-ups with owners |
| Guide | How something works or how to get something done | Who it's written for |
| Reference | A document that lives somewhere else, such as a wiki page or vendor docs | The link (required), where it lives, who can open it |
| Fact | One short thing that's true about the company | The question it answers, when it was last known true, where it came from |
| Brand voice | How the company sounds | 3 or 4 traits, each with what it isn't, and real samples to imitate |
| Writing style | Rules writing can be checked against | Which English, a published guide to fall back on, terms to prefer or avoid |
| Design system | Where the design system lives and which version is current | Links to Storybook, Figma, tokens, packages or its own MCP server, platforms, themes |

A design system entry points to its sources rather than copying them. Storybook, Figma and your token files stay the source of truth, and agents are sent there. A reference works the same way for any single document.

An incident entry is the record you keep afterwards. The live response stays in your incident tool.

Rejected decisions stay findable, so nobody proposes the same thing again.

## Write down an entry

1. Open **Knowledge** and select **New entry**.
2. Pick the kind you're writing down.
3. Enter a **Title**: what people would search for to find it, such as "Expenses policy" or "Orders pipeline on-call".
4. Enter a **Summary** of 1 or 2 sentences. Lists and search results show it, and agents read it first, so make it say what the entry decides.
5. Add **Owners**. They keep the entry true and get its review reminders.
6. Fill in any details the kind requires, then select **Create policy** (or the kind you picked).

The entry opens with its write-up already outlined. Write under each heading, and change the headings if they don't fit.

To change the details later, select **Edit details** on the entry.

## Status and review

Every entry has a status that tells people and agents whether to rely on it:

- **In use**: the current answer. Most entries are **Active**, a decision is **Accepted** and an incident write-up is **Published**.
- **Not in use yet**: shown, but not treated as the current answer. Drafts, entries **In review** and proposed decisions are here.
- **No longer in use**: kept for the record, listed after current entries. Superseded, retired, rejected and deprecated entries are here.

Set **In force from** when an entry applies from a later date, such as a policy that starts next quarter. Until that date it counts as not in use yet, and its page says when it comes into force.

Set **Review by** to the date an owner should check the entry is still true. Once that date passes, the entry shows **Review due** and appears under **Needs attention** on the Knowledge home. An owner selects **Mark reviewed** to confirm it and set the next review date.

### Replace an entry

When a new entry replaces an old one, set **Replaces** on the new entry to the old one. Both must be the same kind. Once the new entry is in use, Marmot marks the old one superseded and its page links to the replacement.

To take an entry out of use without replacing it, set its status to **Retired** rather than deleting it. **Delete entry** removes the entry, its details and its write-up for everyone.

## Link an entry to what it applies to

An entry applies to assets in 3 ways, set under **Links** on the entry:

- **Linked directly**: select **Add an asset** and search the catalog. Pick how the entry relates to it: **Applies to** (it governs the asset), **Documents** (it explains the asset, such as a decision record for a pipeline it shaped) or **Related to** (worth reading alongside it).
- **Matched by a rule**: select **Add rule** and match every asset with a **Tag**, of a **Type** or from a **Provider**, such as everything tagged `pii`. Rules cover assets added later, so use them rather than linking assets one by one.
- **From a container**: an asset inherits the entries linked to whatever contains it, up to 5 levels up. A policy linked to a database applies to every schema and table inside it, and a design system linked to a repository applies to the apps inside it.

Containment comes from the `CONTAINS` relationships your plugins and lineage record. An entry linked to a part doesn't apply to the whole, so a runbook linked to one table doesn't show on its database.

## Find what applies to an asset

On any asset's page, **Knowledge that applies here** lists the entries that cover it, with entries in use first. Each says how it applies: linked directly, matched by a rule or from the container it came through.

To see them all, select **See all** under the list. The page that opens lists every entry about the asset, and you can filter by kind, status, **How it applies** and **Owning team**.

To look something up without starting from an asset, search from the Knowledge home or select **Browse all**. Search matches titles, summaries, details and write-ups, and finds words as you type them, so "expen pol" finds the expenses policy.

## From the CLI

The [CLI](cli.md#marmot-knowledge) reads and writes entries too, which suits keeping runbooks or decisions in a repository and syncing them:

```bash
# See a kind's fields and statuses
marmot knowledge kinds decision

# Write one down, with its write-up from a file
marmot knowledge create --kind decision --title "Count MRR from paid invoices" \
  --summary "MRR is counted from paid recurring invoice lines." \
  --owner "team:Billing" --field decided_at=2026-04-02 --field deciders="Billing,Data platform" \
  --writeup-file decision.md --documents mrn://asset/table/bigquery/analytics.marts.fct_mrr

# Replace an older decision, which becomes superseded
marmot knowledge update "Count MRR from paid invoices" --supersedes "Count MRR from subscriptions"

# Confirm it's still true and set the next review
marmot knowledge reviewed "Count MRR from paid invoices" --next 2027-04-01

# What applies to a table
marmot knowledge about mrn://asset/table/bigquery/analytics.marts.fct_mrr
```

Add `-o json` to any of them for output a script can read.

## How agents use knowledge

An assistant connected to the [MCP server](MCP/index.md) gets these tools for knowledge:

- `search_knowledge` finds entries by words or kind, or, given a repository, service or table, every entry that applies to it, including inherited and rule-matched ones.
- `read_knowledge` reads one entry in full: its status, owners, details, what it covers and its write-up. Long write-ups come back in parts by section.
- `create_knowledge` saves something to knowledge, such as the steps that fixed an incident. It's only offered to identities with `assets:manage`. The assistant shows a preview first and saves only after the user approves it, the entry starts as a draft for someone to review, and text that looks like a password or key is refused.

The server tells connected assistants to check knowledge before they write docs, copy, UI or code for your company, and to say which entries they relied on. Superseded and retired entries are left out of searches unless the assistant asks for them, and reading one names the entry that replaced it.

Agents treat write-ups as information from your company, not as instructions about their tools or permissions. A write-up can't widen what an agent's identity is allowed to do.

## Permissions

Reading entries needs `assets:view`. Creating, editing, linking and marking entries reviewed needs `assets:manage`, the same as editing any other asset.

## Next steps

- [MCP server](MCP/index.md): connect an assistant so it reads your knowledge before it writes.
- [Glossary](glossary.md): define the business terms your entries use.
- [Asset rules](asset-rules.md): tag assets automatically, so knowledge rules that match by tag cover them.
