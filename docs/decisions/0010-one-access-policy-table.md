# 10. One access policy table, closed by default

Status: Accepted

## Context

A rule per feature drifts, and a feature added without one ends up open.

## Decision

Who may do what is one model (`services/backend/internal/entity/access.go`). Every resource (a profile, a note) declares an access mode for each action (`read`, `create`, `update`, `delete`). There are two modes: public (everybody, signed in or not) and private (the owner alone). One table holds every pair, and a pair it doesn't declare is refused.

- A note picks its own read audience: its `visibility` is the two modes read back as one, and anything unrecognised reads as private.
- There are no roles. Every rule is decided by who owns a thing, and a role would sit between that rather than answer it.
- A resource shared with named readers would add a third mode (the owner plus whoever is named) to the table, not a rule of its own.
- Something stored under another resource (a comment under a note, say) is governed by its parent's read rule first, asked in order, rather than by a mode of its own.

## Consequences

A new resource or action needs a line in the table, or it is refused. `services/backend/README.md` explains how a permission is asked and why a refused read is a `404`, not a `403`.
