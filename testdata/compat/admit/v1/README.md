# testdata/compat/admit/v1

Frozen recordings of claims that older releases emit, with the decision the admit step (ADR-0002, rules R4 and R5) must give in four phases. This directory is never regenerated: a recording changes only when a later ADR supersedes the rule it pins, and then a `v2` is added beside it.

`cases.json` holds, for a fixed secret and the keys named in it, the addresses each key derives (nonce 0 and nonce 1; the default subnet is `10.42.0.0/16` from subnet bytes 42 and 7, the custom one `192.168.100.0/24`). The values were computed by hand from the derivation formulas (sha256 of `key+secret` for the natural address, the length-prefixed `nonce=` form for a nonce) and are checked against today's code by `TestAdmitFixtureMatchesDerivation` in `pkg/daemon`.

Cases: a rotated address (inside the mesh network, derived by no key), an address in `10.0.0.0/8` outside the mesh network that no key derives to, a collision-nonce address, a custom-subnet address, a node without routes, a hijack (a member claims the address another key derives), a hijack of an address its holder derives with nonce 1 (the holder is bound at a nonce above 0), a rotated node already in the store against a newcomer whose natural address equals the rotated one, routes inside the mesh network and equal to a member's address, and the supernets `0.0.0.0/0`, `::/0`, `10.0.0.0/8`.

Phases (`TestAdmitCompatFixtures`, `pkg/node/admit_compat_test.go`): `warn` (both switches false, release N), `ip` (`EnforceIPBinding` only, N+1), `both` (N+2), `ip-then-both` (the offer under `ip`, then the route switch flipped and the same offer again: the recorded decision is the second one). A case flagged `never_refused` (rotated, nonce, custom-subnet and supernet claims) must be admitted in every phase; the case flagged `hijack` must be refused once `EnforceIPBinding` is on.

A warning in `expect` is a rule that would act under enforcement and is counted (`Policy.OnWarn`) only while its switch is false; an enforced rule acts and does not warn.

## Grinding measurement (ADR-0002 Open Question 1, task T6 step S6)

`WGMESH_MEASURE=1 go test ./pkg/node -run TestMeasureBindingGrinding -v`, 2026-11-15, host `mars` (Intel Core i5-9300H, 4 cores, Linux 6.17), Go 1.27.4: key pairs a member must generate until one derives, with nonce 0 in the default subnet, to a chosen victim's address. 20 trials: mean 61004, median 51345, min 4232, max 173042 (2 min 25 s in all). Keys are made in process (X25519 on random bytes, base64 as `wg pubkey` prints them). The nonce 1 to 10 variants of the victim's address are not measured here; the 20-trial result is consistent with the ADR estimate of about 65 thousand tries for nonce 0.
