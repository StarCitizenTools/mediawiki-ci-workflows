# AGENTS.md

## Overview

Reusable GitHub Actions CI workflows for MediaWiki skins and extensions. See `README.md` for workflow documentation, inputs, examples, and caching strategy.

## Setup

Enable the pre-commit hook:

```sh
git config core.hooksPath .githooks
```

Install linters:

- **actionlint**: https://github.com/rhysd/actionlint#install
- **yamllint**: `pip install yamllint`

## Verification

**Always run linters before committing.** The pre-commit hook runs them automatically, but you can also run them manually:

```sh
actionlint
yamllint -c .yamllint.yml .github/workflows/
```

To verify workflow behavior, push to a branch and trigger a CI run in a caller repo (e.g. Citizen) that references it via `@<branch>`.

## Conventions

### Workflow design

- Every workflow uses `workflow_call` — they are not standalone
- Inputs use descriptive names with sensible defaults so callers can be minimal
- Job names are set in the reusable workflow (callee), not the caller — GitHub Actions displays `<caller-key> / <callee-name>`
- Boolean inputs default to `false`; required string inputs have no default
- Secrets are declared at workflow level and scoped to the specific step that needs them

### Commits

- Use [Conventional Commits](https://www.conventionalcommits.org/) (e.g. `fix:`, `feat:`, `docs:`)
- Do **not** include emojis in commit messages

### Adding a new workflow

1. Create `.github/workflows/<name>.yml` with `workflow_call` trigger
2. Define inputs with descriptions and defaults
3. Add a job with a descriptive `name:` (this becomes the display suffix in caller repos)
4. Document the workflow in `README.md` — inputs table, description, and example
