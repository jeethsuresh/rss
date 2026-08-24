# CODEX Agent Primer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create a concise root-level launchpad that lets GPT-5.6 Sol work safely and autonomously in this repository.

**Architecture:** `CODEX.md` summarizes operational invariants and routes the agent to authoritative repository documents. It adds no runtime code or duplicated product specification.

**Tech Stack:** Markdown, Bun workspace scripts, Go toolchain, Git

## Global Constraints

- Direct user instructions and `AGENTS.md` remain authoritative.
- Keep the primer concise, concrete, and free of placeholders.
- Do not change runtime behavior.

---

### Task 1: Add and validate the agent launchpad

**Files:**
- Create: `CODEX.md`

**Interfaces:**
- Consumes: `AGENTS.md`, `README.md`, `docs/architecture.md`, `TODO.md`, and root package scripts.
- Produces: A self-contained first-turn workflow and repository map for GPT-5.6 Sol.

- [x] **Step 1: Write `CODEX.md`**

Include the mission, source-of-truth order, architecture boundaries, code map, first-turn launch sequence, implementation rules, verification commands, shipping workflow, and definition of done from the approved design.

- [x] **Step 2: Validate references and commands**

Run:

```bash
test -f AGENTS.md &&
test -f docs/architecture.md &&
test -f TODO.md &&
test -f backend/go.mod &&
bun run backend:build
```

Expected: exit code 0 and a rebuilt `apps/desktop/resources/bin/rss-backend`.

- [x] **Step 3: Self-review**

Check that `CODEX.md` contains no `TBD` or `TODO` placeholders, stays concise, and does not contradict `AGENTS.md` or `docs/architecture.md`.

- [x] **Step 4: Commit**

```bash
git add CODEX.md \
  docs/superpowers/specs/2026-08-24-codex-agent-primer-design.md \
  docs/superpowers/plans/2026-08-24-codex-agent-primer.md
git commit -m "docs: add GPT-5.6 Sol repository launchpad"
```
