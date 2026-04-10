# Tf-Dev Wrapper Reference

Complete reference for the `tf-dev` wrapper script. Full output is written to temp files; only summaries are printed.

---

## Prohibition

> **NEVER run raw `terraform plan`, `terraform apply`, `terraform destroy`, or `terraform init`.**
> Always use `tf-dev`. Wrapper location: `~/.claude/bin/tf-dev` or in PATH.

## Commands

```bash
tf-dev init                        # Initialize (provider versions + status)
tf-dev validate                    # Validate config (errors only on failure)
tf-dev fmt                         # Format check (dry run, list files)
tf-dev fmt-fix                     # Apply formatting
tf-dev plan                        # Plan with file-based output (summary + path)
tf-dev plan -target=aws_instance.x # Plan specific resource
tf-dev plan-json                   # Plan and save JSON for scanning
tf-dev apply                       # Apply with auto-approve (file output)
tf-dev apply-plan tfplan           # Apply saved plan (safest)
tf-dev destroy                     # Destroy with auto-approve (file output)
tf-dev state-list                  # List resources in state
tf-dev state-show aws_s3_bucket.b  # Show single resource details
tf-dev test                        # Run terraform test (pass/fail summary)
tf-dev check                       # Full verification: validate + fmt + plan
```

## Workflows

### Standard: Plan -> Review -> Apply
```bash
tf-dev plan                    # Summary + temp file path
# Read the temp file to inspect changes
tf-dev apply                   # Or tf-dev apply-plan tfplan
```

### Saved Plan (Safest)
```bash
tf-dev plan -out=tfplan        # Create saved plan
# Read temp file to review
tf-dev apply-plan tfplan       # Apply exact plan
```

### JSON Plan (For Scanning)
```bash
tf-dev plan-json               # Creates tfplan + tfplan.json
# Scan with sec-scan or Grep
tf-dev apply-plan tfplan
```

## Apply Safety

- `tf-dev apply` and `tf-dev destroy` pass `-auto-approve` with bold warning header
- `tf-dev apply-plan` does NOT need `-auto-approve` (saved plans are pre-approved)
- Recommended: always `tf-dev plan` -> review -> `tf-dev apply-plan tfplan`

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `TF_DEV_VERBOSE=1` | Show more output lines |
| `TF_DEV_RAW=1` | Unfiltered output (use sparingly) |
| `TF_DEV_MAX_LINES=N` | Override default line limit (50) |

## Fallback

Only use raw commands when wrapper is confirmed unavailable at `~/.claude/bin/tf-dev` AND not in PATH AND user explicitly approves. Then: `terraform plan -no-color 2>&1 | head -50`

## Not Wrapped

`terraform import`, `terraform state mv/rm`, `terraform workspace`, `terraform console` -- run directly when needed.
