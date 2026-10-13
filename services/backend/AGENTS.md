# services/backend

The Go API on Cloud Run. `README.md` here has the API, configuration, how to run it locally, and how to add a resource by copying `notes`. Reasoning for each rule is in the linked record under `docs/decisions/`.

## Access Control

- Every resource and action is a line in `internal/entity/access.go`'s policy table, with one of two modes: public or private. An undeclared pair is refused ([10](../../docs/decisions/0010-one-access-policy-table.md)).
- No roles ([10](../../docs/decisions/0010-one-access-policy-table.md)).
- A refused read is a `404`, not a `403` (see `README.md`).

## Adding a Resource

- Copy the `notes` slice end to end: proto, entity, repository interface, Firestore adapter, handlers, tests, and a composite index in `infrastructure/env/firestore.tf` for any query that filters and orders on different fields (`README.md`'s "Adding a resource").
- Validate through entity setters; a repository calls `Validate` before every write and never trusts what it was handed.
- A resource an account owns is erased by `DELETE /users/me`.

## Rate Limiting

- Every route is behind the per-IP budget, and every write behind the per-account one. A route that calls something paid gets its own, tighter budget.
- Rate-limit buckets are per instance: revisit them before running more than one instance.

## Container image

- `services/backend:image` builds the Dockerfile with `COMMIT_SHA`; CI builds it on every PR but only publishes on merge ([12](../../docs/decisions/0012-hand-declared-moon-graph.md)).
- A second binary goes under `cmd/`, built from the same Dockerfile with `CMD=<name>`, as its own moon project and image matrix leg ([12](../../docs/decisions/0012-hand-declared-moon-graph.md)).
