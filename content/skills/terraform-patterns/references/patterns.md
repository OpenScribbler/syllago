# Terraform Patterns

Production-ready Terraform patterns for writing and structuring HCL configurations. For module-specific design, see [module-design.md](module-design.md).

---

## Provider Configuration

- Rule: Pin provider versions with `version = "~> X.0"` in `required_providers`. Pin Terraform itself with `required_version = "~> 1.7"`.
- Rule: Use `default_tags` block in AWS provider for automatic tagging (`Environment`, `ManagedBy`, `Project`).
- Rule: Remote backend with encryption and locking.

## Variable Design

- Rule: Every variable needs `description`, `type`, and `validation` block for constrained inputs.
- Rule: Use `optional()` with defaults in object variables to avoid requiring callers to specify every field.
- Rule: Mark secrets `sensitive = true`. Name booleans `enable_*` or `create_*`.
- Rule: Avoid `type = any` -- use explicit types for safety and IDE support.
- Gotcha: Locals should only hold computed/derived values (`"${var.project}-${var.env}"`), never just rename a variable.

### Validation Patterns

```hcl
# Enum validation
validation {
  condition     = contains(["dev", "staging", "prod"], var.environment)
  error_message = "Must be dev, staging, or prod."
}

# Regex validation
validation {
  condition     = can(regex("^[a-z][a-z0-9-]{2,28}[a-z0-9]$", var.name))
  error_message = "Must be 4-30 chars, lowercase alphanumeric with hyphens."
}

# Cross-field validation (object variable)
validation {
  condition     = var.scaling.min_capacity <= var.scaling.max_capacity
  error_message = "min must be <= max."
}

# List validation with alltrue
validation {
  condition     = alltrue([for cidr in var.cidrs : can(cidrhost(cidr, 0))])
  error_message = "All entries must be valid CIDR blocks."
}
```

## Resource Patterns

- Rule: Use `for_each` over `count` for named resources. `count` causes index-shift issues on complex resources.
- Rule: Conditional creation: `count = var.enable_x ? 1 : 0`. For `for_each` alternative: `for_each = var.enable_x ? { "key" = true } : {}`.
- Rule: Use locals for computed values (`name_prefix`, `common_tags` via `merge()`). Define data sources once in `data.tf`.
- Rule: Reference conditional resources safely: `var.enable_x ? resource.name[0].id : null`.
- Rule: Use `one(resource.optional[*].id)` for single-or-null references.

### Lifecycle Rules

- `prevent_destroy = true` -- databases, storage, stateful resources
- `create_before_destroy = true` -- zero-downtime compute replacement
- `ignore_changes = [attr]` -- attributes managed outside Terraform
- `replace_triggered_by = [resource]` -- controlled replacement

### Dynamic Blocks

- Rule: Use `dynamic` blocks for repeated nested blocks. Limit nesting to 2 levels max.
- Rule: If nesting exceeds 2 levels, extract to locals or a purpose-built module.

## Lookup Map Pattern (Replacing Nested Ternaries)

Non-obvious -- separate mapping data from selection logic:

```hcl
locals {
  instance_types = {
    prod-default = "m5.xlarge"
    prod-memory  = "r5.2xlarge"
    staging      = "t3.large"
    dev          = "t3.small"
  }
  instance_key  = var.environment == "prod" ? "prod-${var.workload_profile}" : var.environment
  instance_type = local.instance_types[local.instance_key]
}
```

## Custom Conditions

| Type | Fails Plan/Apply? | Use For |
|------|-------------------|---------|
| `variable validation` | Yes (plan) | Input format/range checks |
| `precondition` | Yes (plan/apply) | Cross-variable or data-dependent validation |
| `postcondition` | Yes (apply) | Validating created resource state |
| `check` block | No (warning only) | Drift detection, advisory monitoring |

- Rule: Use `precondition` in `lifecycle` to enforce cross-variable constraints (e.g., `var.environment != "prod" || var.enable_monitoring`).
- Rule: Use `postcondition` to validate resource state after creation (e.g., `self.multi_az == true` for prod databases).
- Rule: Use `check` blocks with `assert` for non-fatal monitoring (e.g., certificate expiry warnings).

## Feature Flags

- Rule: Use `enable_*` booleans with `count` or `for_each` for conditional resources.
- Rule: Group multiple feature flags into an object variable with `optional()` defaults.
- Rule: For environment-based flags, use a `feature_defaults` map in locals keyed by environment name.

## Provider Aliasing (Multi-Region/Multi-Account)

Non-obvious -- modules must declare `configuration_aliases`:

```hcl
# Root module: define and pass aliased providers
provider "aws" { alias = "us_east"; region = "us-east-1" }
provider "aws" { alias = "eu_west"; region = "eu-west-1" }

module "global" {
  source    = "./modules/global"
  providers = { aws.primary = aws.us_east, aws.secondary = aws.eu_west }
}

# modules/global/main.tf: declare expected aliases
terraform {
  required_providers {
    aws = { source = "hashicorp/aws", configuration_aliases = [aws.primary, aws.secondary] }
  }
}
```

- Rule: For cross-account, use `assume_role` in the aliased provider block.

## Environment Management

| Scenario | Approach |
|----------|----------|
| Environments differ only in size/count | Workspaces work |
| Different providers/accounts per env | Use directories |
| Team > 5 engineers | Use directories (explicit is safer) |
| Dynamic/ephemeral environments | Workspaces work well |

- Rule: Directory-per-environment (`environments/dev/`, `environments/prod/`) is the default recommendation. Each directory has own `main.tf`, `backend.tf`, `terraform.tfvars`, calling shared modules.
- Gotcha: Don't use workspaces when environments need different provider configurations.

## Dependency Management

| Pattern | Coupling | Best For |
|---------|----------|----------|
| Data sources | Low | Existing infra lookups (preferred) |
| Remote state | High | Tightly coupled configs (avoid if possible) |
| SSM/Parameter Store | Low | Cross-team dependencies |

- Rule: Validate data source lookups with `check` blocks.
- Rule: Use SSM Parameter Store or Consul for cross-team dependencies instead of `terraform_remote_state`.

## Ephemeral Resources (Terraform 1.10+)

- Rule: Use `ephemeral` resources for secrets that should not persist in state. Fetched every plan/apply, never stored.
- Gotcha: Cannot be referenced by non-ephemeral outputs.
- Gotcha: Support depends on provider.

## Useful Functions Reference

| Function | Purpose |
|----------|---------|
| `coalesce(a, b)` | First non-empty value |
| `try(expr, fallback)` | Graceful fallback for optional fields |
| `one(list)` | Single element or null |
| `flatten(list)` | Flatten nested lists |
| `merge(map1, map2)` | Merge maps (last wins) |
| `lookup(map, key, default)` | Safe map access |
| `cidrsubnet(prefix, bits, num)` | Subnet math |
| `alltrue(list)` / `anytrue(list)` | Validation conditions |

## Standard File Structure

| File | Contents |
|------|----------|
| `versions.tf` | `terraform {}`, `required_providers` |
| `main.tf` | Primary resources, `provider` blocks |
| `variables.tf` | All `variable` blocks |
| `outputs.tf` | All `output` blocks |
| `locals.tf` | Computed values, merged tags |
| `data.tf` | Data sources |
| `<service>.tf` | Resources for a specific service (large configs) |

- Rule: Use `snake_case` consistently for all Terraform identifiers.
- Rule: Commit `.terraform.lock.hcl`. Only `.terraform/` directory goes in `.gitignore`.
