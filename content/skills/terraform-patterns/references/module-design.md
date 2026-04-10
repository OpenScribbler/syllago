# Terraform Module Design

Patterns for designing, structuring, and versioning Terraform modules. For testing modules, see [testing.md](testing.md).

---

## When to Create a Module

| Scenario | Recommendation |
|----------|---------------|
| Used in 2+ places | Create a module |
| 5+ tightly coupled resources | Create a module (encapsulation) |
| Team-shared infrastructure pattern | Create a module (governance) |
| Simple 1-2 resources, used once | Keep inline |
| Wrapping a single resource for defaults | Create wrapper module (enforce standards) |

Rule of thumb: if you copy a block of resources between configs, extract to a module.

## Composition Patterns

| Pattern | Flexibility | Testability | Best For |
|---------|------------|-------------|----------|
| **Flat modules** | High | High | Most projects (recommended default) |
| **Nested modules** | Low | Medium | Always-together components |
| **Wrapper modules** | Medium | High | Org standards enforcement |

- **Flat**: Each module manages one concern. Root wires outputs to inputs.
- **Nested**: Parent calls child modules, exposes simplified interface. Use when combined interface is simpler than sum of parts.
- **Wrapper**: Thin module wrapping a resource with org-specific defaults (encryption, tagging). Single place to update standards.

## Interface Design

For variable and output design rules, see [patterns.md](patterns.md).

## File Structure

| File | Contents |
|------|----------|
| `main.tf` | Resources, module calls |
| `variables.tf` | All variables (required first, then optional) |
| `outputs.tf` | All outputs (one per resource identifier minimum) |
| `versions.tf` | `required_version` + `required_providers` |
| `locals.tf` | Computed values, name prefixes, merged tags |
| `data.tf` | External lookups |

Additionally: `README.md`, `examples/basic/`, `examples/complete/`, `tests/` directory.

### versions.tf Guidance

- Rule: Use `>=` lower bound (not `~>`) for modules consumed by others -- avoid over-constraining consumers. Root configs should use `~>` to pin.

## Versioning Strategies

| Strategy | Source Syntax | Best For |
|----------|-------------|----------|
| Git tags | `git::https://...//module?ref=v1.2.0` | Private modules |
| Terraform Registry | `source = "org/name/provider"` + `version = "~> 1.0"` | Open source modules |
| Local path | `source = "./modules/network"` | Monorepo, app-specific modules |

### Version Constraints

| Constraint | Meaning |
|-----------|---------|
| `= 1.2.3` | Exact version |
| `~> 1.2` | `>= 1.2, < 2.0` (allow minor/patch) |
| `~> 1.2.0` | `>= 1.2.0, < 1.3.0` (allow patch only) |
| `>= 1.0` | Minimum only (best for modules) |

## Documentation

- Rule: Use `terraform-docs` to auto-generate README sections. Configure with `.terraform-docs.yml`.
- Rule: Provide `examples/basic/` (minimal) and `examples/complete/` (all features) directories.

## Module Design Checklist

- [ ] Single responsibility (one logical concern)
- [ ] All variables have `description` and `type`
- [ ] Required variables have `validation` blocks
- [ ] Sensitive variables/outputs marked `sensitive = true`
- [ ] All resource IDs/ARNs in outputs
- [ ] `versions.tf` with `required_version` and `required_providers`
- [ ] Boolean variables named `enable_*` or `create_*`
- [ ] No provider configuration inside the module
- [ ] Uses `for_each` over `count` for named resources
- [ ] `README.md`, `examples/`, and `tests/` directories
