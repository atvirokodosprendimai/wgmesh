# Compatibility Tracking

API compatibility checks require an `openapi.yaml` file in the repository. The reusable workflow compares the current OpenAPI document with the document from the configured base ref and fails on breaking changes.

Call the workflow from a repository workflow like this:

```yaml
name: API Compatibility

on:
  pull_request:

jobs:
  oasdiff:
    uses: ./.github/workflows/oasdiff-template.yml
    with:
      openapi-path: openapi.yaml
      fail-on-warning: false
```

## Hard-Block Policy

Error-level breaking changes are hard blocks and must be fixed before merge. Warning-level changes are advisory by default, but API-owning branches can set `fail-on-warning: true` when they need stricter enforcement. Do not bypass a hard block by removing the workflow or weakening the base ref.

## Ignore Rules

False positives should be handled with an explicit ignore file and a short explanation in the PR. Add the narrowest regex possible, include the affected endpoint or schema name, and remove the rule when the compatibility exception expires. Ignore rules should be reviewed by the API owner before merge.
