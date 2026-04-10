# Terraform Refactoring Patterns

Safely refactoring, restructuring, and migrating Terraform configurations using declarative blocks.

---

## Refactoring Tools

| Tool | Version | Purpose |
|------|---------|---------|
| `moved` block | 1.1+ | Rename/restructure resources in code |
| `import` block | 1.5+ | Declaratively import existing resources |
| `removed` block | 1.7+ | Remove from state without destroying |
| `terraform state mv` | All | Imperatively move (escape hatch) |

**Rule: Prefer declarative blocks** over CLI commands. They are version-controlled, reviewable, and reproducible.

## Decision Tree

- Rename a resource -> `moved` block
- Move resource into/out of a module -> `moved` block
- Import existing infrastructure -> `import` block + `-generate-config-out`
- Stop managing a resource (keep infra) -> `removed { lifecycle { destroy = false } }`
- Move between configurations -> `removed` (source) + `import` (destination)
- Refactor `count` to `for_each` -> `moved` blocks mapping indices to keys
- Handle provider resource rename -> `moved` (if supported) or `removed` + `import`

## Moved Blocks (1.1+)

```hcl
# Rename resource
moved { from = aws_instance.main; to = aws_instance.app }

# Extract into module
moved { from = aws_vpc.main; to = module.networking.aws_vpc.main }

# count to for_each (one per instance)
moved { from = aws_subnet.private[0]; to = aws_subnet.private["us-east-1a"] }
```

- Gotcha: Keep `moved` blocks until all environments have applied.
- Gotcha: Always verify `terraform plan` shows "has moved to" with zero destroy/create.

## Import Blocks (1.5+)

```hcl
import { to = aws_instance.legacy; id = "i-0abc123def456" }

# for_each import (1.7+)
import {
  for_each = { "app-sg" = "sg-0abc123", "db-sg" = "sg-0def456" }
  to = aws_security_group.imported[each.key]
  id = each.value
}
```

### Generated Config Workflow

1. Write `import` blocks only (no resource blocks)
2. `terraform plan -generate-config-out=generated.tf`
3. Refine: remove computed attrs, replace hardcoded values with variables, add lifecycle rules
4. `terraform plan` -- should show "no changes"
5. `terraform apply`, then remove `import` blocks

### Common Import ID Formats

| Provider | Resource | ID Format |
|----------|----------|-----------|
| AWS | `aws_instance` | `i-xxx` |
| AWS | `aws_s3_bucket` | bucket name |
| AWS | `aws_iam_role` | role name |
| AWS | `aws_security_group_rule` | `sg-id_type_protocol_from_to_source` |
| Azure | Most resources | Full Resource ID (`/subscriptions/.../...`) |
| GCP | `google_compute_instance` | `projects/p/zones/z/instances/name` |

Each resource type has a specific format -- check provider docs "Import" section.

## Removed Blocks (1.7+)

```hcl
removed { from = aws_instance.legacy; lifecycle { destroy = false } }
```

- Rule: Works for modules too: `from = module.old_monitoring`.
- Rule: Keep until all state files updated.

## Splitting Large Configurations

| Signal | Action |
|--------|--------|
| Plan > 2 minutes | Split by lifecycle |
| > 200 resources | Split by domain |
| Multiple teams editing | Split by ownership |

Use `removed` in old config + `import` in new config. Apply old first (releases state), then new (imports).

## Provider Upgrade Workflow

1. Read provider CHANGELOG/UPGRADE guide
2. Step upgrades (don't skip major versions)
3. `terraform init -upgrade`
4. Fix deprecation warnings / breaking changes
5. Verify plan shows only expected changes

### Common Breaking Changes

- **Attribute renamed**: Update attribute name
- **Resource split** (e.g., S3 inline to separate resources): Create sub-resources + `import`
- **Resource renamed**: `moved` block (if supported), otherwise `removed` + `import`

## Safety Checklist

- [ ] `terraform plan` reviewed completely before applying
- [ ] Zero resource destroys unless intentional
- [ ] `moved` shows "has moved to", `import` shows "will be imported"
- [ ] Tested in non-production environment first
- [ ] State backed up before state manipulation
- [ ] Refactoring documented in commit message / PR
