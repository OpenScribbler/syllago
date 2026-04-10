# Terraform Anti-Patterns

Design-level code smells for **reviewing** Terraform configurations. For security-specific anti-patterns, see [security-scanning.md](security-scanning.md).

---

## Configuration

### Deeply Nested Ternaries
**Severity**: high

- Rule: No ternary nesting beyond 2 levels. Unreadable and error-prone.
- Fix: Use lookup maps to separate data from logic (see patterns.md "Lookup Map Pattern").

### Triple-Nested Dynamic Blocks
**Severity**: high

- Rule: Limit dynamic blocks to 2 levels. Beyond that, extract to locals or a purpose-built module.

### Provisioners for Configuration Management
**Severity**: high

- Rule: Provisioners are not idempotent. Use `user_data`, pre-built AMIs (Packer), or containers instead.

### Hardcoded IDs
**Severity**: medium

- Rule: Never hardcode AMI IDs, account IDs, or regions. Use `data` sources (`aws_ami`, `aws_caller_identity`, `aws_region`) or variables.

## Module Design

### God Module
**Severity**: high

- Rule: Modules with 20+ variables, 3+ services, or 500+ lines should be split into focused modules composed at root level.

### Thin Wrapper Module
**Severity**: medium

- Rule: If variables map 1:1 to resource arguments with no defaults, validation, or additional resources, the module adds no value. Good modules add opinionated defaults, security baselines, or compose multiple resources.

### Circular Dependencies
**Severity**: high

- Rule: If Module A needs Module B's output and vice versa, extract the shared concern to a third module.

### Unpinned Module Sources
**Severity**: high

- Rule: Pin registry modules with `version = "~> X.0"`, git sources with `?ref=vX.Y.Z`.

## Variables

### Pass-Through Variables
**Severity**: medium

- Rule: 15+ variables that map 1:1 to resource args indicate missing abstraction. Provide opinionated abstractions (e.g., t-shirt sizes instead of raw instance types).

### Missing Validation
**Severity**: medium

- Rule: Add `validation {}` for enums, CIDRs, email formats, and other constrained types. Catch errors at plan time, not apply time.

### type = any
**Severity**: medium

- Rule: Use explicit types (`string`, `map(string)`, `object({...})`). `any` gives no IDE support or validation.

## Operations

### Local State in Team Environment
**Severity**: high

- Rule: Use remote backend (S3+DynamoDB, GCS, Terraform Cloud) with encryption and locking.

### No Version Pinning
**Severity**: high

- Rule: Pin providers with `version = "~> X.0"` and Terraform with `required_version = "~> 1.7"`.

### Not Committing .terraform.lock.hcl
**Severity**: medium

- Rule: Commit lock file for reproducible builds. Only `.terraform/` directory goes in `.gitignore`.

### Deprecated Arguments
**Severity**: medium

- Rule: Replace deprecated arguments with current API. Warnings now become breakage later.

## Performance

### Monolithic State
**Severity**: high

- Rule: Split by lifecycle/team when plan takes >30s, state >5MB, or different teams own different resources.
- Split domains: `networking/`, `database/`, `application/`, `monitoring/`.

### Unnecessary Data Source Lookups
**Severity**: low

- Rule: Use direct resource references (`aws_vpc.main.id`) instead of data source lookups for resources you created in the same configuration.

### Not Preallocating for_each
**Severity**: medium

- Rule: Excessive `for_each` (100+ resources) causes state bloat. Use higher-level constructs (IAM groups instead of per-user resources, ASGs instead of individual instances).

## State Management

### Manual State Editing
**Severity**: high

- Rule: Never edit state JSON directly. Use `terraform state mv`, `terraform state rm`, `terraform import`, or declarative blocks (`moved`, `import`, `removed`).

### Missing State Locking
**Severity**: high

- Rule: Always configure state locking (DynamoDB for S3 backend). Concurrent applies without locking corrupt state.
