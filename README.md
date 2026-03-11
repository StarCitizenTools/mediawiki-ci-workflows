# mediawiki-ci-workflows

Reusable GitHub Actions CI workflows for MediaWiki skins and extensions.

## Prerequisites

Your project needs:

- **PHP**: `composer.json` with `test` (PHPCS), `phan`, and PHPUnit scripts
- **JS**: `package.json` with `lint:js`, `lint:styles`, `lint:i18n`, `lint:md` scripts and Vitest
- **Coverage**: Vitest outputs to `coverage/js/lcov.info`; PHPUnit outputs Clover XML

## Secrets

| Secret | Required | Used by | Setup |
|--------|----------|---------|-------|
| `SONAR_TOKEN` | No | `sonarqube.yml` | [SonarCloud](https://sonarcloud.io) > Your project > Administration > Analysis Method |

All workflows use public GitHub Actions and public MediaWiki archives — no other secrets are needed.

## Workflows

### `lint.yml` — Linting

Runs PHP, JS, style, i18n, and markdown linters, each toggled independently.

**Inputs:**

| Input | Type | Default | Description |
|-------|------|---------|-------------|
| `lint-php` | boolean | `false` | Run `composer test` |
| `lint-js` | boolean | `false` | Run `npm run lint:js` |
| `lint-styles` | boolean | `false` | Run `npm run lint:styles` |
| `lint-i18n` | boolean | `false` | Run `npm run lint:i18n` |
| `lint-md` | boolean | `false` | Run `npm run lint:md` |

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

Downloads a MediaWiki branch, installs your project into it, and runs `composer phan`.

**Inputs:**

| Input | Type | Default | Description |
|-------|------|---------|-------------|
| `project-type` | string | *required* | `skin` or `extension` |
| `project-name` | string | *required* | Directory name (e.g. `Citizen`, `TabberNeue`) |
| `mw-branch` | string | `REL1_43` | MediaWiki branch |
| `php-version` | string | `8.2` | PHP version |
| `skip-cache` | boolean | `false` | Skip MW cache (for nightly runs) |

```yaml
analyze-php:
  uses: StarCitizenTools/mediawiki-ci-workflows/.github/workflows/analyze-php.yml@main
  with:
    project-type: skin
    project-name: Citizen
```

---

### `test-js.yml` — JavaScript tests

Runs Vitest with coverage and caches the results for SonarQube.

**Inputs:** None.

```yaml
test-js:
  uses: StarCitizenTools/mediawiki-ci-workflows/.github/workflows/test-js.yml@main
```

---

### `test-php.yml` — PHPUnit tests

Runs PHPUnit across a matrix of MediaWiki branches and PHP versions. Set `coverage: "xdebug"` on a matrix entry to generate coverage for SonarQube.

**Inputs:**

| Input | Type | Default | Description |
|-------|------|---------|-------------|
| `project-type` | string | *required* | `skin` or `extension` |
| `project-name` | string | *required* | Directory name (e.g. `Citizen`, `TabberNeue`) |
| `matrix` | string | *(see below)* | JSON array of matrix entries |
| `skip-cache` | boolean | `false` | Skip MW cache (for nightly runs) |

**Default matrix:**

```json
[
  {"mw": "REL1_43", "php": "8.2", "coverage": "xdebug", "experimental": false},
  {"mw": "REL1_44", "php": "8.3", "coverage": "none",   "experimental": false},
  {"mw": "REL1_45", "php": "8.4", "coverage": "none",   "experimental": false},
  {"mw": "master",  "php": "8.5", "coverage": "none",   "experimental": true}
]
```

| Field | Description |
|-------|-------------|
| `mw` | MediaWiki branch (`REL1_43`, `REL1_44`, `REL1_45`, `master`) |
| `php` | PHP version |
| `coverage` | `xdebug` to generate coverage, `none` to skip |
| `experimental` | `true` allows failure without failing the workflow |

```yaml
test-php:
  uses: StarCitizenTools/mediawiki-ci-workflows/.github/workflows/test-php.yml@main
  with:
    project-type: skin
    project-name: Citizen
```

To customize the matrix:

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

### `sonarqube.yml` — SonarQube analysis

Runs a SonarQube scan with coverage data from `test-js` and `test-php`. Coverage is cached per branch, so SonarQube always has data for both JS and PHP even when only one test suite ran.

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

Your project also needs a `sonar-project.properties` file. See [SonarCloud docs](https://docs.sonarsource.com/sonarcloud/advanced-setup/ci-based-analysis/github-actions-for-sonarcloud/).

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
  group: ${{ github.workflow }}-${{ github.event_name }}-${{ github.ref }}
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

## Caching

| What | Cache key | Shared across | Refreshed by |
|------|-----------|---------------|--------------|
| MediaWiki installation | `mw-<branch>-php<version>` | `test-php` and `analyze-php` | `skip-cache: true` (nightly) |
| Composer packages | `composer-php<version>` | All MW branches | Automatic (Composer) |
| JS coverage | `coverage-js-<branch>` | SonarQube runs | Each `test-js` run |
| PHP coverage | `coverage-php-<branch>` | SonarQube runs | Each `test-php` run |
