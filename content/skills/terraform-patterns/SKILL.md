---
name: terraform-patterns
description: Terraform and Infrastructure as Code patterns. Use when writing or reviewing Terraform configurations, including provider-specific patterns for Aembit, AWS, Azure, GCP, and others.
---

# Terraform Patterns

This skill provides patterns for writing production-quality Terraform configurations.

## Quick Reference

| Category | Best Practice |
|----------|---------------|
| Secrets | Never hardcode -- use variables or data sources |
| Versions | Pin provider versions (`~> 1.0`) and Terraform (`~> 1.7`) |
| State | Remote backend with encryption and locking |
| Modules | Flat composition, pin versions, single responsibility |
| Naming | `snake_case` for all identifiers |
| Testing | Native `terraform test` for unit/contract; Terratest for integration |
| Security | Scan with Trivy or Checkov in CI; `sensitive = true` on secrets |

## Provider Versioning Rules

| Scenario | Rule |
|----------|------|
| **New Terraform** | Use latest stable provider versions |
| **Existing Terraform** | Do NOT update provider versions unless explicitly instructed |

## Terraform Wrapper

> **NEVER run raw `terraform plan`, `terraform apply`, etc.**
> Always use `tf-dev`. See [tf-dev-wrapper.md](references/tf-dev-wrapper.md) for full reference.

```bash
tf-dev check     # validate + fmt + plan
tf-dev plan      # Plan with summary output
tf-dev apply     # Apply with auto-approve
```

## References

Load on-demand based on task:

| When to Use | Reference |
|-------------|-----------|
| Full tf-dev commands, plan/apply workflow, fallback | [tf-dev-wrapper.md](references/tf-dev-wrapper.md) |
| Writing HCL: variables, resources, validation, lifecycle, feature flags, provider aliasing, environments | [patterns.md](references/patterns.md) |
| Module design: composition, interfaces, versioning, file structure, documentation | [module-design.md](references/module-design.md) |
| Code smells: god modules, hardcoded IDs, pass-through vars, monolithic state, missing validation | [anti-patterns.md](references/anti-patterns.md) |
| Renaming, importing, migrating, splitting configs, provider upgrades, `moved`/`import`/`removed` blocks | [refactoring.md](references/refactoring.md) |
| Writing tests: `terraform test`, mock providers, Terratest, terraform-compliance, test organization | [testing.md](references/testing.md) |
| Security scanning: Trivy, Checkov, OPA/Conftest, pre-commit hooks, drift detection, security anti-patterns | [security-scanning.md](references/security-scanning.md) |
| Aembit provider: workload IAM, trust providers, credential providers, access policies | [providers/aembit.md](references/providers/aembit.md) |

## Related Skills

- **Aembit knowledge**: Load [aembit-knowledge](../aembit-knowledge/SKILL.md) for Aembit concepts and API details
- **Code review**: Load [code-review-standards](../code-review-standards/SKILL.md) when reviewing Terraform
- **Kubernetes**: Load [kubernetes-patterns](../kubernetes-patterns/SKILL.md) for K8s manifests that pair with Terraform
