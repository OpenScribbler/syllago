# Hook Policy Interface

This document defines the interface contract for hook execution policies. It specifies the fields and their semantics but does not define the full policy enforcement mechanism, configuration format, or distribution model. Those are deferred to a future Hook Policy Specification.

This interface ships alongside the [Hook Interchange Format Specification v1](hooks-v1.md) because enterprise adopters need to understand the policy surface area before adopting the hook manifest format.

---

## 1. Purpose

Hook manifests define **what a hook does**. Hook policies define **whether a hook is allowed to run**. These are separate concerns:

- A hook author writes a manifest declaring events, handlers, and matchers.
- An organization defines a policy controlling which hooks execute in their environment.

The policy interface describes the control points that policy systems MAY implement. Implementations are not required to support all fields; this interface defines the vocabulary for interoperability between policy systems and hook runtimes.

---

## 2. Policy Fields

### 2.1 `disable_all_hooks`

| Property | Value |
|----------|-------|
| Type | boolean |
| Default | `false` |
| Scope | Global |

When `true`, no hooks execute regardless of other policy settings. This is a kill switch for environments where hook execution is not permitted.

### 2.2 `allow_managed_only`

| Property | Value |
|----------|-------|
| Type | boolean |
| Default | `false` |
| Scope | Global |

When `true`, only hooks distributed through managed channels (organization registries, signed packages, MDM-deployed configurations) are permitted to execute. Hooks from local files, project repositories, or unmanaged sources are blocked.

The definition of "managed" is implementation-specific. Implementations SHOULD document their criteria for classifying a hook source as managed.

### 2.3 `capability_restrictions`

| Property | Value |
|----------|-------|
| Type | object |
| Default | `{}` (no restrictions) |
| Scope | Per-capability |

An object keyed by capability identifier (Section 9 of the specification). Each value is a restriction rule:

```json
{
  "capability_restrictions": {
    "input_rewrite": "deny",
    "http_handler": "managed_only",
    "llm_evaluated": "deny",
    "async_execution": "allow"
  }
}
```

Restriction values:

| Value | Meaning |
|-------|---------|
| `allow` | Hooks using this capability may execute without additional checks. |
| `deny` | Hooks using this capability MUST NOT execute. |
| `managed_only` | Hooks using this capability may execute only if they come from a managed source. |

When a capability is not listed, it defaults to `allow`.

### 2.4 `event_restrictions`

| Property | Value |
|----------|-------|
| Type | object |
| Default | `{}` (no restrictions) |
| Scope | Per-event |

An object keyed by canonical event name. Each value is a restriction rule using the same vocabulary as `capability_restrictions`:

```json
{
  "event_restrictions": {
    "before_model": "deny",
    "permission_request": "managed_only",
    "session_start": "allow"
  }
}
```

When an event is not listed, it defaults to `allow`.

---

## 3. Evaluation Order

When multiple policy fields apply to a single hook, they are evaluated in the following order. The first `deny` result stops evaluation.

1. `disable_all_hooks` -- if `true`, deny.
2. `allow_managed_only` -- if `true` and the hook is not from a managed source, deny.
3. `event_restrictions` -- if the hook's event is `deny`, deny. If `managed_only` and the hook is not managed, deny.
4. `capability_restrictions` -- for each inferred capability, if any is `deny`, deny. If any is `managed_only` and the hook is not managed, deny.
5. If no rule denied the hook, allow.

---

## 4. Scope and Precedence

Policies MAY be defined at multiple scopes:

| Scope | Example Location | Precedence |
|-------|-----------------|------------|
| System | `/etc/<tool>/policy.json` | Highest |
| Organization | MDM-deployed, cloud dashboard | High |
| User | `~/.<tool>/policy.json` | Medium |
| Project | `./<tool>/policy.json` | Lowest |

Higher-precedence policies override lower-precedence ones. A system-level `disable_all_hooks: true` cannot be overridden by a project-level `disable_all_hooks: false`.

The specific file paths, merge semantics, and distribution mechanisms are implementation-defined and outside the scope of this interface contract.

---

## 5. Future Work

The full Hook Policy Specification will define:

- Complete policy document format and schema
- Policy distribution and update mechanisms
- Audit logging requirements for policy decisions
- Revocation of previously-allowed hooks
- Integration with identity providers for `managed_only` source verification
- Policy reporting (which hooks were allowed/denied and why)
