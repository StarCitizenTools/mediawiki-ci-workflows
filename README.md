# mediawiki-ci-workflows

Reusable GitHub Actions CI workflows for MediaWiki skins and extensions.

## Prerequisites

Your project must follow MediaWiki's standard tooling conventions:

- **PHP**: `composer.json` with `test` script (PHPCS), `phan` script, and PHPUnit configured
- **JS**: `package.json` with `lint:js`, `lint:styles`, `lint:i18n`, `lint:md` scripts and Vitest for testing
- **Coverage**: Vitest outputs to `coverage/js/lcov.info`; PHPUnit outputs Clover XML

## Secrets

| Secret | Required | Used by | How to get it |
|--------|----------|---------|---------------|
| `SONAR_TOKEN` | No | `sonarqube.yml` | [SonarCloud](https://sonarcloud.io) > Your project > Administration > Analysis Method |

No other secrets are needed. All workflows use only public GitHub Actions and public MediaWiki archives.

## Workflows

### `lint.yml` — Code linting

Runs PHP, JS, style, i18n, and markdown linters. Each linter is toggled independently.

**Inputs:**

| Input | Type | Default | Description |
|-------|------|---------|-------------|
| `lint-php` | boolean | `false` | Run `composer test` |
| `lint-js` | boolean | `false` | Run `npm run lint:js` |
| `lint-styles` | boolean | `false` | Run `npm run lint:styles` |
| `lint-i18n` | boolean | `false` | Run `npm run lint:i18n` |
| `lint-md` | boolean | `false` | Run `npm run lint:md` |

**Example:**

```yaml
lint:
  uses: StarCitizenTools/mediawiki-ci-workflows/.github/workflows/lint.yml@main
  with:
    lint-php: true
    lint-js: true
    lint-styles: true
    lint-i18n: true
    lint-md: true
```

---

### `analyze-php.yml` — Phan static analysis

Runs Phan against a MediaWiki installation. Downloads the specified MW branch, installs your project into it, and runs `composer phan`.

**Inputs:**

| Input | Type | Default | Description |
|-------|------|---------|-------------|
| `project-type` | string | *required* | `skin` or `extension` |
| `project-name` | string | *required* | Directory name (e.g. `Citizen`, `TabberNeue`) |
| `mw-branch` | string | `REL1_43` | MediaWiki branch for analysis |
| `php-version` | string | `8.2` | PHP version |
| `skip-cache` | boolean | `false` | Skip MW cache (use for nightly runs) |

**Example:**

```yaml
analyze-php:
  uses: StarCitizenTools/mediawiki-ci-workflows/.github/workflows/analyze-php.yml@main
  with:
    project-type: skin
    project-name: Citizen
```

---

### `test-js.yml` — JavaScript tests

Runs `npx vitest run --coverage` and saves coverage to a branch-scoped cache for SonarQube.

**Inputs:** None.

**Example:**

```yaml
test-js:
  uses: StarCitizenTools/mediawiki-ci-workflows/.github/workflows/test-js.yml@main
```

---

### `test-php.yml` — PHPUnit tests

Runs PHPUnit against a matrix of MediaWiki branches and PHP versions. One matrix entry can enable Xdebug coverage, which is saved to a branch-scoped cache for SonarQube.

**Inputs:**

| Input | Type | Default | Description |
|-------|------|---------|-------------|
| `project-type` | string | *required* | `skin` or `extension` |
| `project-name` | string | *required* | Directory name (e.g. `Citizen`, `TabberNeue`) |
| `matrix` | string | *(see below)* | JSON array of matrix entries |
| `skip-cache` | boolean | `false` | Skip MW cache (use for nightly runs) |

**Default matrix:**

```json
[
  {"mw": "REL1_43", "php": "8.2", "coverage": "xdebug", "experimental": false},
  {"mw": "REL1_44", "php": "8.3", "coverage": "none",   "experimental": false},
  {"mw": "REL1_45", "php": "8.4", "coverage": "none",   "experimental": false},
  {"mw": "master",  "php": "8.5", "coverage": "none",   "experimental": true}
]
```

Each entry has:

| Field | Description |
|-------|-------------|
| `mw` | MediaWiki branch (`REL1_43`, `REL1_44`, `REL1_45`, `master`) |
| `php` | PHP version |
| `coverage` | `xdebug` to generate coverage, `none` to skip |
| `experimental` | `true` to allow failure without failing the workflow |

**Example:**

```yaml
test-php:
  uses: StarCitizenTools/mediawiki-ci-workflows/.github/workflows/test-php.yml@main
  with:
    project-type: skin
    project-name: Citizen
```

To override the matrix (e.g. drop master or change coverage):

```yaml
test-php:
  uses: StarCitizenTools/mediawiki-ci-workflows/.github/workflows/test-php.yml@main
  with:
    project-type: extension
    project-name: TabberNeue
    matrix: >
      [
        {"mw": "REL1_43", "php": "8.2", "coverage": "xdebug", "experimental": false},
        {"mw": "REL1_44", "php": "8.3", "coverage": "none",   "experimental": false}
      ]
```

---

### `sonarqube.yml` — SonarQube / SonarCloud analysis

Restores cached coverage from `test-js` and `test-php`, then runs the SonarQube scan. Coverage is branch-scoped — if only JS or only PHP tests ran, stale coverage from the other is restored from cache so SonarQube always has the most recent data for both.

**Inputs:**

| Input | Type | Default | Description |
|-------|------|---------|-------------|
| `has-js-coverage` | boolean | `false` | Whether JS coverage was generated this run |
| `has-php-coverage` | boolean | `false` | Whether PHP coverage was generated this run |
| `enabled` | boolean | `true` | Set `false` to skip (e.g. if no `SONAR_TOKEN`) |

**Secrets:**

| Secret | Required | Description |
|--------|----------|-------------|
| `SONAR_TOKEN` | No | SonarCloud/SonarQube authentication token |

**Example:**

```yaml
sonarqube:
  needs: [test-js, test-php]
  if: always() && !failure()
  uses: StarCitizenTools/mediawiki-ci-workflows/.github/workflows/sonarqube.yml@main
  with:
    has-js-coverage: ${{ needs.test-js.result == 'success' }}
    has-php-coverage: ${{ needs.test-php.result == 'success' }}
  secrets:
    SONAR_TOKEN: ${{ secrets.SONAR_TOKEN }}
```

Your project also needs a `sonar-project.properties` file in the repo root. See [SonarCloud docs](https://docs.sonarsource.com/sonarcloud/advanced-setup/ci-based-analysis/github-actions-for-sonarcloud/).

## Full example

A complete caller workflow with change detection, conditional jobs, and nightly cache refresh:

```yaml
name: CI

on:
  schedule:
    - cron: "0 0 * * *"
  push:
    branches: [main]
  pull_request:
    branches: ["**"]

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

jobs:
  changes:
    name: Detect changes
    runs-on: ubuntu-latest
    timeout-minutes: 5
    outputs:
      php: ${{ steps.filter.outputs.php_any_changed }}
      script: ${{ steps.filter.outputs.script_any_changed }}
      stylesheet: ${{ steps.filter.outputs.stylesheet_any_changed }}
      i18n: ${{ steps.filter.outputs.i18n_any_changed }}
      markdown: ${{ steps.filter.outputs.markdown_any_changed }}
    steps:
      - uses: actions/checkout@v6
      - uses: tj-actions/changed-files@v47
        id: filter
        with:
          files_yaml: |
            php:
              - includes/**/*.php
              - tests/**/*.php
              - skin.json        # or extension.json
              - composer.json
              - composer.lock
              - .phan/config.php
              - .github/workflows/ci.yml
            script:
              - resources/**/*.js
              - tests/vitest/**
              - vitest.config.js
              - package.json
              - package-lock.json
              - .eslintrc.json
              - .github/workflows/ci.yml
            stylesheet:
              - resources/**/*.css
              - resources/**/*.less
              - skinStyles/**/*.css
              - skinStyles/**/*.less
              - package.json
              - package-lock.json
              - .github/workflows/ci.yml
            i18n:
              - i18n/*.json
              - package.json
              - package-lock.json
              - .github/workflows/ci.yml
            markdown:
              - "*.md"
              - .markdownlint.json
              - .github/workflows/ci.yml

  lint:
    needs: changes
    if: >-
      needs.changes.outputs.php == 'true' ||
      needs.changes.outputs.script == 'true' ||
      needs.changes.outputs.stylesheet == 'true' ||
      needs.changes.outputs.i18n == 'true' ||
      needs.changes.outputs.markdown == 'true' ||
      github.event_name == 'schedule'
    uses: StarCitizenTools/mediawiki-ci-workflows/.github/workflows/lint.yml@main
    with:
      lint-php: ${{ needs.changes.outputs.php == 'true' || github.event_name == 'schedule' }}
      lint-js: ${{ needs.changes.outputs.script == 'true' || github.event_name == 'schedule' }}
      lint-styles: ${{ needs.changes.outputs.stylesheet == 'true' || github.event_name == 'schedule' }}
      lint-i18n: ${{ needs.changes.outputs.i18n == 'true' || github.event_name == 'schedule' }}
      lint-md: ${{ needs.changes.outputs.markdown == 'true' || github.event_name == 'schedule' }}

  analyze-php:
    needs: changes
    if: needs.changes.outputs.php == 'true' || github.event_name == 'schedule'
    uses: StarCitizenTools/mediawiki-ci-workflows/.github/workflows/analyze-php.yml@main
    with:
      project-type: skin       # or 'extension'
      project-name: MySkin     # your project's directory name
      skip-cache: ${{ github.event_name == 'schedule' }}

  test-js:
    needs: changes
    if: needs.changes.outputs.script == 'true' || github.event_name == 'schedule'
    uses: StarCitizenTools/mediawiki-ci-workflows/.github/workflows/test-js.yml@main

  test-php:
    needs: changes
    if: needs.changes.outputs.php == 'true' || github.event_name == 'schedule'
    uses: StarCitizenTools/mediawiki-ci-workflows/.github/workflows/test-php.yml@main
    with:
      project-type: skin       # or 'extension'
      project-name: MySkin     # your project's directory name
      skip-cache: ${{ github.event_name == 'schedule' }}

  sonarqube:
    needs: [test-js, test-php]
    if: always() && !failure()
    uses: StarCitizenTools/mediawiki-ci-workflows/.github/workflows/sonarqube.yml@main
    with:
      has-js-coverage: ${{ needs.test-js.result == 'success' }}
      has-php-coverage: ${{ needs.test-php.result == 'success' }}
    secrets:
      SONAR_TOKEN: ${{ secrets.SONAR_TOKEN }}
```

## Caching strategy

- **MediaWiki installation** is cached per branch+PHP version (`mw-REL1_43-php8.2`). Nightly runs skip cache to pick up upstream changes — pass `skip-cache: true`.
- **Composer cache** is cached per PHP version (`composer-php8.2`), shared across MW branches.
- **Coverage data** is cached per branch (`coverage-js-main`, `coverage-php-main`). This ensures SonarQube always has coverage even when only one test suite runs.
