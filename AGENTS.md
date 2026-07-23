# Codex Code Guidelines

## Backend Go Rewrite Tracking (Critical)

- The canonical backend Python -> Go migration tracker is `docs/backend-python-to-go-refactor.md`.
- Every backend refactor phase must be marked in that document when the phase starts, blocks, resumes, or completes.
- A phase is not complete until `docs/backend-python-to-go-refactor.md` records the phase status, completion date, verification commands, verification results, deliverables, and residual risks.
- If implementation changes API behavior, database schema, deployment behavior, or migration scope, update the refactor document in the same change.
- If the document cannot be updated, stop and report the reason before claiming the phase is complete.

## Code Quality Standards

### Code Quality

- Follow the project's existing patterns.
- Match import style and naming conventions.
- Keep each function or class focused on one responsibility.
- Apply DRY where it removes real duplication.
- Apply YAGNI and avoid speculative abstractions.
- Prefer boring, simple solutions over clever code.
- Avoid unrelated refactors while working on migration phases.

### Testing

- Test case source files are local, temporary verification artifacts and must not be kept or committed. This includes `*_test.go`, `*.test.*`, `*.spec.*`, `test_*.py`, `*_test.py`, `__tests__/`, `test/`, and `tests/`.
- The absence of test source files in the repository or a pull request is expected. Do not report it as evidence that the author did not test the change or as a review finding by itself.
- Reviewers may run builds, static checks, runtime checks, or their own temporary tests, but must not infer the submitter's local test coverage from the repository tree.
- Delete any locally created temporary test sources and test-only fixtures before staging or committing. Test runner configuration and dependencies may remain.

### Error Handling

- Use proper error handling at module boundaries.
- Provide clear error messages.
- Degrade gracefully where the product can continue safely.
- Do not expose sensitive information in responses or logs.

## Core Principles

### Incremental Progress

- Make small, independently verifiable changes.
- Commit or checkpoint working code frequently when git metadata is available.
- Build on previous subtasks and keep context continuous.

### Evidence-Based Work

- Study 3+ similar patterns before implementing.
- Match project style exactly.
- Verify behavior with existing code, builds, static checks, runtime checks, or documented contracts.

### Pragmatic Execution

- Adapt to project reality.
- Choose maintainable implementation over novelty.
- Keep migration work aligned with the documented phase plan.

### Context Continuity

- Reuse established migration decisions.
- Maintain module boundaries already documented in the migration tracker.
- Verify integration between completed phases and newly migrated phases without requiring test artifacts to be committed.

## Git Operations and Parallel Task Safety

- Only stage or commit files directly produced by the current task.
- Use `git add <specific-files>` instead of `git add .`.
- Verify staged files before committing.
- Never stage or commit test case source files or test-only fixtures; remove them after their verification run.
- Never touch unrelated changes or other task outputs.
- Treat pre-existing uncommitted changes as intentional work in progress.
- If the task conflicts with existing uncommitted changes, stop and report the conflict instead of overwriting.
- If git metadata is unavailable, record modified files explicitly in the final response.

## System Optimization

- Prefer direct binary calls with an explicit working directory.
- Use `apply_patch` for routine text edits.
- Use single-line substitutions only for small mechanical changes.
- Avoid Python editing scripts unless simpler editing tools are unavailable or unsafe.

## Context Acquisition Priority

When MCP code-discovery tools are available, prefer them in this order:

1. `mcp__ace-tool__search_context` for semantic code discovery.
2. `smart_search` for structured keyword, regex, or file discovery.
3. `read_file` for batch file reading.
4. Shell commands as fallback.

If those MCP tools are unavailable in the current environment, use fast shell tools such as `rg` and keep the search scoped.

## Execution Checklist

### Before

- Understand the purpose and task clearly.
- Gather context before editing.
- Find 3+ existing patterns where practical.
- Check relevant rules, templates, and constraints.

### During

- Follow existing patterns.
- Use verification appropriate to the change without requiring test sources to be part of the deliverable.
- Keep the migration tracker current for backend Go rewrite work.

### After

- Confirm no test case source file is staged or tracked before handoff.
- Ensure expected deliverables are complete.
- Update documentation and phase status when a migration phase changes.
