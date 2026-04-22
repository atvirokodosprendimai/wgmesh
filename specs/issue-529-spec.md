# Specification: Issue #529

## Classification
feature

## Problem Analysis

Issue #529 targets Stage 3 (Reachable): users can find wgmesh but cannot complete a billing-backed signup flow.

Current repository state confirms this gap:

1. Billing is explicitly marked as missing in public stage tracking:
   - `docs/index.html` shows revenue stage blocked with `0 — needs billing`.
   - `docs/index.html` NOW roadmap includes `Billing integration — Polar.sh or Stripe`.
2. Customer account storage is not billing-capable:
   - `pkg/mesh/account.go` stores only `api_key` and optional `lighthouse_url`.
3. CLI has no billing/signup/invoice/account-management commands:
   - `main.go` usage text contains `join`, `status`, `service`, `peers`, etc., but nothing for billing lifecycle.
4. Existing external API client has no billing primitives:
   - `lighthouse-go@v0.1.0` provides site CRUD only (`/v1/sites`), no customer signup or invoices.

To satisfy acceptance criteria without depending on unavailable backend changes, billing must be implemented in wgmesh CLI with Stripe API integration, persisted local account metadata, and tested end-to-end with mocked Stripe HTTP endpoints.

## Implementation Tasks

### Task 1: Extend account persistence model to include billing metadata

Modify `pkg/mesh/account.go` and `pkg/mesh/account_test.go`.

Required changes:

1. Add a nested billing section to `AccountConfig`:
   - `email`
   - `name`
   - `plan`
   - `stripe_customer_id`
   - `stripe_default_invoice_id`
   - `billing_enabled` (boolean)
   - `updated_at` (RFC3339 timestamp string)
2. Keep JSON backward compatibility:
   - Existing `account.json` files with only `api_key` must still load successfully.
   - Empty/missing billing fields must not break service commands.
3. Keep secure file permissions and atomic write behavior unchanged (`0600`, tmp+rename).
4. Add tests for:
   - loading legacy account JSON (no billing fields)
   - saving/loading account JSON with billing fields
   - preserving existing `api_key` behavior

### Task 2: Add Stripe billing client package (no new external dependency)

Create a new package under `pkg/billing`.

Create files:
- `stripe.go`
- `stripe_test.go`

Implementation requirements:

1. Use `net/http` + `application/x-www-form-urlencoded` requests to Stripe REST API.
2. Do not add Stripe SDK dependency; keep `go.mod` unchanged.
3. Add client methods:
   - `CreateCustomer(email, name, meshID, plan)`
   - `CreateMonthlyInvoice(customerID, amountCents, currency, description)`
   - `SendInvoice(invoiceID)`
   - `ListInvoices(customerID, limit)`
4. Stripe API details:
   - Auth: `Authorization: Bearer <secret-key>`
   - Base URL default: `https://api.stripe.com` (allow override for tests)
   - Customer endpoint: `POST /v1/customers`
   - Invoice endpoint: `POST /v1/invoices`
   - Invoice-send endpoint: `POST /v1/invoices/{id}/send`
   - Invoice list endpoint: `GET /v1/invoices?customer=...&limit=...`
5. Include robust error handling:
   - parse non-2xx response body and include context in returned error
   - never log Stripe secret key
6. Add unit tests with `httptest.Server` covering success and failure responses for all methods.

### Task 3: Add billing CLI command group with signup, invoice, and account views

Create `billing.go` and wire it from `main.go`.

Required command structure:

1. `wgmesh billing signup`
   - Flags:
     - `--secret <SECRET>` (required; supports `WGMESH_SECRET` fallback)
     - `--email <email>` (required)
     - `--name <name>` (required)
     - `--plan <starter|pro|team>` (default `starter`)
     - `--state-dir` (default `/var/lib/wgmesh`)
   - Environment:
     - `WGMESH_STRIPE_SECRET_KEY` (required)
     - `WGMESH_STRIPE_BASE_URL` (optional test override)
   - Flow:
     1. derive mesh ID from secret (`crypto.DeriveKeys(...).MeshID()`)
     2. load existing account config if present, otherwise initialize empty config
     3. create Stripe customer
     4. generate and send first monthly invoice
     5. persist billing metadata into `account.json`
     6. print signup summary including customer ID and invoice ID

2. `wgmesh billing account`
   - Flags: `--state-dir`, `--json`
   - Reads `account.json` and prints billing/account summary.
   - Must fail with clear message if account not configured.

3. `wgmesh billing invoices`
   - Flags: `--state-dir`, `--limit` (default 10), `--json`
   - Requires configured Stripe customer ID in `account.json` and `WGMESH_STRIPE_SECRET_KEY`.
   - Lists recent invoices from Stripe.

Main wiring requirements:

1. Add `case "billing": billingCmd(); return` in `main()` switch.
2. Update `printUsage()` with billing subcommands and required env vars.
3. Keep existing commands and behavior unchanged.

### Task 4: Define plan-to-price mapping and deterministic invoice generation behavior

Implement plan constants in `billing.go` (or `pkg/billing/pricing.go` if preferred).

Required mapping:

- `starter` → 500 EUR cents/month
- `pro` → 2500 EUR cents/month
- `team` → 10000 EUR cents/month

Rules:

1. Invalid plan must return explicit error and non-zero exit.
2. Invoice description format:
   - `wgmesh <plan> plan - mesh <meshID> - monthly subscription`
3. Currency fixed to `eur` for this phase.
4. Signup must be idempotent-safe:
   - if local account already has a Stripe customer ID, do not create a second customer unless `--force` is explicitly passed.
   - add `--force` flag to `billing signup` to allow re-provisioning.

### Task 5: Add end-to-end CLI tests for billing signup and invoice retrieval

Create `billing_test.go`.

Required test scenarios (use mocked Stripe HTTP server):

1. `TestBillingSignupCreatesCustomerAndInvoice`
   - execute signup flow
   - assert `account.json` contains billing metadata and customer/invoice IDs
2. `TestBillingSignupRequiresStripeKey`
   - missing `WGMESH_STRIPE_SECRET_KEY` must fail with clear error
3. `TestBillingSignupRejectsInvalidPlan`
4. `TestBillingInvoicesUsesPersistedCustomerID`
5. `TestBillingSignupIdempotentWithoutForce`

Testing constraints:

1. Build and execute the CLI binary in tests (same pattern as `main_test.go` / existing CLI integration tests).
2. Use temporary state directory only.
3. Do not require root privileges.

### Task 6: Add operator documentation for billing workflow

Update `README.md`.

Add a new section `## Billing (Stage 3 Reachable)` with:

1. Required environment variable setup:
   - `export WGMESH_STRIPE_SECRET_KEY=...`
2. Signup command example.
3. Account info command example.
4. Invoice list command example.
5. Short note that Stripe API keys and customer PII must not be committed to git.

### Task 7: Validation checklist for implementation PR

After implementation, Goose must run and include output evidence for:

1. `go test ./pkg/mesh -run Test.*Account`
2. `go test ./pkg/billing -run Test`
3. `go test ./... -run TestBilling`
4. `go test ./...`

Manual verification commands (in PR description):

1. `wgmesh billing signup --secret "wgmesh://v1/<secret>" --email "user@example.com" --name "User" --plan starter --state-dir <tmpdir>`
2. `wgmesh billing account --state-dir <tmpdir>`
3. `wgmesh billing invoices --state-dir <tmpdir> --limit 5`

Acceptance criteria mapping:

- Payment integration functional → Stripe client + signup flow tests
- Customer can sign up and receive invoice → billing signup creates and sends invoice
- Basic account management in place → `billing account` + persisted billing metadata
- Billing system tested end-to-end → CLI e2e tests with mocked Stripe
