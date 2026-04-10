# BuildDemo Workflow

> **Trigger:** "build demo", "create demo", "lambda graph demo", "/demobuilder"

## Purpose

Interactively collect configuration and generate a complete Terraform project for the Lambda → Microsoft Graph demo.

## Prerequisites

- AWS account with Lambda permissions
- Azure AD tenant with app registration permissions
- Aembit tenant configured

## Interactive Flow

### Phase 1: Collect Configuration

Use `AskUserQuestion` to collect these inputs **one at a time**:

| Order | Input | Type | Default | Validation |
|-------|-------|------|---------|------------|
| 1 | Output directory | path | (required) | Create if not exists |
| 2 | Demo prefix | string | "aembit-demo" | Alphanumeric + hyphen |
| 3 | AWS region | choice | "us-east-1" | Valid AWS region |
| 4 | AWS account ID | string | (required) | 12-digit number |
| 5 | Azure tenant ID | string | (required) | Valid UUID |
| 6 | Aembit tenant ID | string | (required) | Non-empty |
| 7 | Aembit OIDC issuer | string | (required) | Valid URL |
| 8 | Aembit Lambda Layer ARN | string | (required) | Valid ARN format |

**Question Examples:**

Question 1: "Where should I generate the demo files?"
- Header: "Output Dir"
- Options: [Custom path input required]

Question 2: "What prefix should I use for resource names?"
- Header: "Prefix"
- Options: ["aembit-demo (Recommended)", "Custom prefix"]

Question 3: "Which AWS region for the Lambda function?"
- Header: "Region"
- Options: ["us-east-1 (Recommended)", "us-west-2", "eu-west-1", "Custom"]

### Phase 2: Validate & Create Output Directory

```
IF output directory doesn't exist:
  Create it with mkdir -p

IF output directory has files:
  Ask: "Directory has existing files. Overwrite?"
  Options: ["Yes, overwrite", "No, choose different directory"]
```

### Phase 3: Generate Files

Generate files in this order using templates from `Templates/` directory:

1. `providers.tf` - Provider configurations
2. `variables.tf` - Input variable definitions
3. `outputs.tf` - Output definitions
4. `01-aws-lambda.tf` - Lambda infrastructure
5. `02-aembit-policy.tf` - Aembit access policy
6. `03-azure-entra.tf` - Azure Entra app registration
7. `lambda/handler.py` - Lambda function code
8. `README.md` - Setup and deployment guide

**Template Substitution:** Replace `{{variable}}` placeholders with collected values:

- `{{demo_prefix}}` - Resource name prefix
- `{{aws_region}}` - AWS region
- `{{aws_account_id}}` - AWS account ID
- `{{azure_tenant_id}}` - Azure tenant ID
- `{{aembit_tenant}}` - Aembit tenant ID
- `{{aembit_oidc_issuer}}` - Aembit OIDC issuer URL
- `{{aembit_lambda_layer_arn}}` - Aembit Lambda Layer ARN

### Phase 4: Report Success

```
✅ Demo files generated successfully!

Output directory: {output_path}

Generated files:
  - README.md (setup guide)
  - providers.tf
  - variables.tf
  - outputs.tf
  - 01-aws-lambda.tf
  - 02-aembit-policy.tf
  - 03-azure-entra.tf
  - lambda/handler.py

Next steps:
  1. cd {output_path}
  2. Review and customize variables.tf
  3. terraform init
  4. terraform plan
  5. terraform apply
```

## Error Handling

| Error | Action |
|-------|--------|
| Invalid AWS account ID | Re-prompt with format hint (12 digits) |
| Invalid UUID | Re-prompt with format hint |
| Cannot create directory | Report error, ask for different path |
| File write fails | Report specific error |
