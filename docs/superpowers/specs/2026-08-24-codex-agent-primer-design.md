# CODEX Agent Primer Design

## Goal

Add a concise root-level `CODEX.md` that lets GPT-5.6 Sol safely begin useful repository work from its first prompt.

## Audience and precedence

The primer targets a capable coding agent with no prior repository context. It is an operational launchpad, not a replacement for `AGENTS.md`, product specs, or implementation plans. Direct user instructions and `AGENTS.md` remain authoritative.

## Content

The primer will fit in roughly one screen of dense Markdown and cover:

- the product mission and local-first architecture;
- non-negotiable ownership and IPC boundaries;
- a map of the directories and documents that answer common questions;
- a first-turn launch sequence, including `um ai` memory retrieval;
- implementation rules for contracts, migrations, security, and scope;
- focused and full verification commands;
- git, shipping, memory-capture, and handoff expectations;
- a concrete definition of done.

It will link to existing source-of-truth documents rather than copying product history.

## Agent workflow

An agent should inspect context before editing, use tests for behavior changes, update both sides of cross-language contracts, and preserve unrelated worktree changes. It should use focused checks during iteration, run the relevant full checks before completion, and rebuild the desktop backend binary before handoff.

## Success criteria

- A new agent can locate the relevant code and source-of-truth documents without broad exploration.
- Architectural constraints are explicit enough to prevent business logic, storage, or RSS fetching from moving into TypeScript.
- Commands match repository scripts.
- The document has no placeholders, stale phase claims, or duplicated long-form specifications.
