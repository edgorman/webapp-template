# 7. Visibility is a string, not an enum

Status: Accepted

## Context

`appv1.Note.visibility` and `NoteRequest.visibility` carry `"public"` or `"private"`. A proto3 `enum` would give a closed set of values on both sides.

## Decision

Keep a plain `string`, for three reasons together:

1. `protojson` serializes an enum by member name, so the wire value would become `VISIBILITY_PUBLIC`: a breaking rename of a value the frontend compares against (`note.visibility === 'private'`), with no bug to fix.
2. proto3 requires a zero value, conventionally `_UNSPECIFIED`, but a visibility has no unspecified state. `entity.Visibility.Valid()` accepts exactly the two values, so an enum would split the two definitions of validity.
3. The server-side safety already exists: `entity.Visibility` is its own type with `Valid()`, and every write goes through `SetVisibility`.

The frontend gets its guarantee by hand: `api.ts` declares `type Visibility = 'public' | 'private'`, rather than deriving the union from `Note['visibility']`, which would silently widen to `string`.

## Consequences

Weaker typing on the wire is the accepted cost; validation still runs in full on every write.
