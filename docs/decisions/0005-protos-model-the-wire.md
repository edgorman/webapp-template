# 5. Protos model the wire, not the domain or the datastore

Status: Accepted

## Context

There are three representations of the same data: `internal/entity` (validation and policy, e.g. `SetTitle`, `Permission`), `internal/repository/firestore`'s `...Document` structs with converters, and `internal/service`, the wire.

## Decision

`packages/protos` defines the wire and nothing else. Entities and Firestore documents stay hand-written.

- One entity can need several wire messages. `appv1.User` has no `id` because a public lookup must never disclose the account's uid; `appv1.CurrentUser` has it, because an account may see its own.
- Wire messages carry what the server resolves: `Note.author_username` has no counterpart on the entity.
- `protojson` can only marshal a message at the top level, so a list gets a wrapper: `NotePage`.
- A field where "absent" and "empty" mean different requests is `optional`: `UpdateCurrentUserRequest.username` (omitted keeps the stored name, `""` is a name the backend rejects), like `ListNotesParams`.
- A route addressed entirely by its URL reads no request message.
- Access control and rate limiting stay out of the protos: a proto models what is exchanged, not who may ask.

## Consequences

Generated structs can't replace entities anyway: they carry `protoimpl` state Firestore would try to persist, and none of the entity methods.
