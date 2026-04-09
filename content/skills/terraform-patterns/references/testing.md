# Terraform Testing Patterns

Testing strategies and patterns for Terraform modules. For security scanning and policy-as-code, see [security-scanning.md](security-scanning.md).

---

## Testing Strategy

| Level | Tool | Speed | Cost |
|-------|------|-------|------|
| Static analysis | `terraform validate`, `terraform fmt` | Seconds | Free |
| Unit | `terraform test` + mock providers | Seconds | Free |
| Contract | `terraform test` + mocks | Seconds | Free |
| Policy | terraform-compliance, OPA/Conftest | Seconds | Free |
| Integration | `terraform test` (apply), Terratest | Minutes | Cloud costs |

**Unit tests (always write)**: variable validation, local computations, conditional creation, output values.
**Integration tests (critical modules)**: real resource creation, security group behavior, cross-resource references.

## Native Test Framework (1.6+)

Test files: `.tftest.hcl` in `tests/` directory. Run with `terraform test` or `tf-dev test`.

### Run Block Reference

```hcl
run "block_name" {
  command = plan                            # plan (default) or apply
  variables { key = "value" }               # Override variables
  module { source = "./modules/networking" } # Test specific module
  providers = { aws = aws.mock }            # Provider overrides

  assert {
    condition     = <boolean expression>
    error_message = "Descriptive failure message"
  }

  expect_failures = [var.environment]       # Negative testing
}
```

### Mock Providers (1.7+)

```hcl
mock_provider "aws" {
  mock_data "aws_ami" {
    defaults = { id = "ami-mock123", architecture = "x86_64" }
  }
  mock_resource "aws_instance" {
    defaults = { id = "i-mock123", private_ip = "10.0.1.100" }
  }
}
```

- Rule: Use mock providers for unit tests (no cloud costs). All resources return zero-values by default.
- Rule: Use aliased mocks (`alias = "mock"`) to combine real and mock providers in the same test file.

### Override Blocks (1.7+)

- `override_resource { target = aws_instance.main; values = { id = "i-override" } }` -- override specific resources
- `override_data { target = data.aws_ami.latest; values = { id = "ami-override" } }` -- override data sources
- `override_module { target = module.vpc; outputs = { vpc_id = "vpc-override" } }` -- override module outputs

### Testing Variable Validation

- Rule: Write both positive (valid input, no `expect_failures`) and negative (invalid input, `expect_failures = [var.name]`) test cases.

### Chaining Run Blocks

- Rule: Run blocks execute sequentially. Reference previous outputs via `run.<block_name>.<output>`.
- Rule: Use `command = apply` + `module { source = ... }` for integration test chains.

## Terratest (Go Integration Testing)

Use when native tests are insufficient: HTTP endpoint validation, retry/wait logic, custom SDK assertions, parallel execution.

```go
func TestModule(t *testing.T) {
    t.Parallel()
    opts := terraform.WithDefaultRetryableErrors(t, &terraform.Options{
        TerraformDir: "../modules/my-module",
        Vars: map[string]interface{}{"name": "test-" + random.UniqueId()},
    })
    defer terraform.Destroy(t, opts)
    terraform.InitAndApply(t, opts)
    assert.Contains(t, terraform.Output(t, opts, "id"), "test-")
}
```

- Rule: Always `defer terraform.Destroy`. Use `random.UniqueId()` for unique names. Call `t.Parallel()`.

## terraform-compliance (BDD Policy Testing)

Gherkin-syntax policy testing. Feature files in `features/` directory.

```gherkin
Scenario: All resources must have required tags
  Given I have resource that supports tags defined
  Then it must contain tags
  And its value must contain "Environment"
```

Run: `terraform plan -out=plan.out && terraform show -json plan.out > plan.json && terraform-compliance -p plan.json -f features/`

## Test Organization

- **Native tests**: `modules/<name>/tests/*.tftest.hcl`
- **Terratest**: `test/` at project root with `go.mod`
- **terraform-compliance**: `features/` at project root

## Testing Checklist

- [ ] Every module has at least one `.tftest.hcl` file
- [ ] Variable validation rules have positive and negative test cases
- [ ] Security properties asserted (encryption, public access)
- [ ] Mock providers used for unit tests
- [ ] Integration tests use unique names and clean up resources
- [ ] CI runs unit tests on every PR, integration tests on demand
