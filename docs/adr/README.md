# Decision records

One record per area; a small change amends the record that owns its area instead of adding a new one. The Status column is each record's own Status line. Deferred work from accepted records is kept in [BACKLOG.md](BACKLOG.md).

## Records

| Record | Area | Status | Decision |
|---|---|---|---|
| [ADR-0001](ADR-0001-envelope-aad/ADR-0001-envelope-aad.md) | pkg/crypto envelope, pkg/discovery exchange | Accepted | Authenticate the envelope type with GCM additional data and refuse replays of non-HELLO datagrams, in a three-release wire rollout |
