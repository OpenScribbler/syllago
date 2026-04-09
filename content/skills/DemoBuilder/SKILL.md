---
name: DemoBuilder
description: Build end-to-end demos for Aembit Technical Solutions team. USE WHEN creating demos OR Aembit Lambda demo OR Graph API demo OR cross-cloud identity demo. Generates Terraform infrastructure (AWS Lambda + Aembit Layer + Azure Entra WIF) and documentation.
---

# DemoBuilder

Build end-to-end demos for the Aembit Technical Solutions team. Currently supports the Lambda → Microsoft Graph demo scenario.

## Workflow Routing

| Workflow | Trigger | File |
|----------|---------|------|
| **BuildDemo** | "build demo", "create demo", "lambda graph demo" | `Workflows/BuildDemo.md` |

## Flags

| Flag | Purpose | Example |
|------|---------|---------|
| `--output` | Specify output directory | `/demobuilder --output ./my-demo` |

## Examples

**Example 1: Build Lambda to Graph demo**
```
User: "/demobuilder --output ./lambda-graph-demo"
→ Invokes BuildDemo workflow
→ Asks for AWS region, account ID, Azure tenant ID, etc.
→ Generates Terraform files and Lambda code to ./lambda-graph-demo/
```

**Example 2: Quick demo generation**
```
User: "Create an Aembit demo for Lambda calling Microsoft Graph"
→ Invokes BuildDemo workflow
→ Asks for output directory
→ Collects configuration interactively
→ Outputs complete Terraform project
```

**Example 3: Show what will be generated**
```
User: "What does the demo builder create?"
→ Lists generated files: README.md, 01-aws-lambda.tf, 02-aembit-policy.tf,
   03-azure-entra.tf, variables.tf, outputs.tf, providers.tf, lambda/handler.py
```
