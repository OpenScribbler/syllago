<!-- modeled after: payloadcms/payload CLAUDE.md -->
## API Reference

This section documents the public API. Internal helpers are intentionally
omitted; they are not covered by semver guarantees and may change without
notice between releases.

### Collections

Collections are the primary persistence unit. Each collection has a
schema and a set of access control rules.

#### find

Returns a paginated result set. Supports filtering, sorting, and depth
control for populating relationships.

#### findByID

Returns a single document. Throws a not-found error if the id does not
resolve to a live document in the collection.

#### create

Inserts a new document. The returned document includes server-assigned
fields like id and timestamps.

#### update

Partial updates are supported via patch semantics. Fields not included
in the request body are left unchanged.

#### delete

Removes the document. Soft delete is available when the collection has
trash enabled.

### Globals

Globals are singletons. They live outside the collection abstraction and
are ideal for site-wide configuration.

#### findGlobal

Reads the singleton. Missing globals return the schema defaults.

#### updateGlobal

Writes the singleton. Revision history is kept for audit purposes.

## Hooks

Hooks run at well-defined points in the request lifecycle. They are the
primary extension point.

### Collection Hooks

Run per-operation and per-collection. Scope to a collection via the
collection slug.

#### beforeChange

Runs before a create or update. The hook may mutate the incoming data
to enforce invariants.

#### afterChange

Runs after a successful write. Ideal for cache invalidation, search
index updates, or webhooks.

### Access Control Hooks

Evaluated on every request. Return true to allow the operation or a
query-like object to restrict the result set.

#### read

Gates reads. The returned query is AND-ed with the caller's query.
