# Terraform Security Scanning

Static analysis, policy-as-code, and security patterns for Terraform. For testing patterns, see [testing.md](testing.md).

---

## Tool Selection

| Need | Recommended |
|------|------------|
| Quick setup, broad coverage | Trivy or Checkov |
| Organization-specific policies | OPA/Conftest |
| Multi-framework scanning | Checkov or KICS |
| Already using OPA | Terrascan or Conftest |
| Compliance frameworks (CIS, SOC2) | Checkov |
| Scanning plan JSON | Checkov, Conftest, or Snyk |

### When to Scan HCL vs Plan JSON

- **HCL**: No credentials needed, fast, catches intent. Use in pre-commit hooks and early CI.
- **Plan JSON**: Resolves all values, most accurate. Use before apply.

## Trivy (Successor to tfsec)

```bash
trivy config .                              # Scan current directory
trivy config --severity HIGH,CRITICAL .     # Filter by severity
trivy config --format sarif .               # For GitHub Security tab
trivy config --ignorefile .trivyignore .    # Use ignore file
```

- Rule: Scan plan JSON for accuracy: `terraform show -json plan.out > plan.json && trivy config plan.json`
- Suppress: `.trivyignore` file with rule IDs, or inline `#trivy:ignore:RULE-ID reason`
- Custom rules: Rego with METADATA comments. Run with `--config-policy ./custom-checks`

## Checkov

```bash
checkov -d .                                # Scan directory
checkov -f plan.json --framework terraform_plan  # Scan plan
checkov -d . --check CKV_AWS_18,CKV_AWS_19      # Specific checks
checkov -d . --skip-check CKV_AWS_18             # Skip checks
checkov -d . --compliance-framework cis_aws      # Compliance
```

- Suppress inline: `#checkov:skip=CKV_AWS_18:Reason`
- Config: `.checkov.yml` with `skip-check`, `soft-fail-on`, `framework` settings
- Custom: Python (`BaseResourceCheck`), YAML (attribute checks), or graph-based (cross-resource relationships)

## OPA / Conftest

```bash
terraform show -json plan.out > plan.json
conftest test plan.json --policy ./policy/
```

- Rule: Access planned resources via `input.planned_values.root_module.resources[_]`.
- Rule: Access changes via `input.resource_changes[_]`.
- Organize policies by domain: `main.rego`, `tags.rego`, `encryption.rego`, `networking.rego`.

## Security Anti-Patterns

### IAM
**Severity**: high

- Rule: Never use `Action = "*"` or `Resource = "*"`. Use specific actions and resource ARNs.

### Missing Encryption
**Severity**: high

- Rule: Set `encrypted = true` / `storage_encrypted = true` on all storage resources. Configure KMS keys.

### Public S3 Buckets
**Severity**: high

- Rule: Always add `aws_s3_bucket_public_access_block` with all four settings `true`.

### Open Security Groups
**Severity**: high

- Rule: No `0.0.0.0/0` on sensitive ports. Restrict to specific ports (443, 80) and add `description` on every rule.

### Secrets in Code
**Severity**: critical

- Rule: No secrets in `.tf` or committed `.tfvars`. Use `data "aws_secretsmanager_secret_version"`, variables with `sensitive = true`, or CI secrets injection.

### Sensitive Outputs
**Severity**: medium

- Rule: Add `sensitive = true` to outputs containing passwords, keys, or tokens.

### Missing Logging
**Severity**: medium

- Rule: Enable logging on S3, CloudTrail, ALB, VPC flow logs, and RDS audit logging.

### Missing S3 Versioning
**Severity**: medium

- Rule: Enable `aws_s3_bucket_versioning` on all data buckets for recovery from accidental deletion.

## Pre-Commit Hooks

```yaml
# .pre-commit-config.yaml (key hooks)
repos:
  - repo: https://github.com/antonbabenko/pre-commit-terraform
    rev: v1.96.1
    hooks:
      - id: terraform_fmt
      - id: terraform_validate
      - id: terraform_tflint
      - id: terraform_trivy
        args: ['--args=--severity HIGH,CRITICAL']
  - repo: https://github.com/gitleaks/gitleaks
    rev: v8.21.2
    hooks:
      - id: gitleaks
```

### tflint Config

```hcl
# .tflint.hcl
plugin "terraform" { enabled = true; preset = "recommended" }
plugin "aws" { enabled = true; version = "0.31.0"; source = "github.com/terraform-linters/tflint-ruleset-aws" }
rule "terraform_naming_convention" { enabled = true; format = "snake_case" }
rule "terraform_documented_variables" { enabled = true }
```

## Severity Levels

| Severity | Action |
|----------|--------|
| CRITICAL | Must fix before merge |
| HIGH | Must fix before merge |
| MEDIUM | Fix or document exception |
| LOW/INFO | Best effort / backlog |

## Drift Detection

- Rule: Schedule `terraform plan -detailed-exitcode` in CI. Exit code 0 = no changes, 1 = error, 2 = drift detected.
- Rule: Notify team on exit code 2.

## Common False Positives

| Scenario | Fix |
|----------|-----|
| Variable-driven encryption (scanner can't resolve) | Suppress with comment |
| Log bucket flagged for no logging | Suppress (it IS the logging bucket) |
| Dev env flagged for no HA | Per-environment scan config |
| Dynamic block values | Scan plan JSON instead of HCL |

## Security Checklist

- [ ] No hardcoded secrets in `.tf` files
- [ ] Sensitive variables/outputs marked `sensitive = true`
- [ ] Provider versions pinned
- [ ] Remote state with encryption and locking
- [ ] IAM policies follow least privilege
- [ ] All storage encrypted at rest
- [ ] S3 public access blocks enabled
- [ ] Security groups restrict ingress
- [ ] Logging enabled on applicable resources
- [ ] Deletion protection on stateful resources
