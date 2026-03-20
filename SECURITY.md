# Security Policy

## Scope

Vulnerabilities we want to hear about:

- **Malicious content injection** -- rules, skills, agents, or hooks that execute attacker-controlled code through syllago's install or apply mechanisms
- **Path traversal** -- add, install, or export operations writing files outside the expected target directory
- **MCP config injection** -- JSON merge inserting unexpected server entries into provider settings
- **Symlink escape** -- content that resolves outside the expected install target via symlink manipulation
- **Registry trust bypass** -- mechanisms to install unconfirmed content without user awareness or confirmation

### Out of Scope (By Design)

- **Hooks and MCP servers execute code by design.** They are shell scripts and server processes. The user is responsible for trusting their source before installing.
- **Third-party registry content.** Syllago does not own, curate, or verify registry content. Registries are git repositories maintained by their owners.

## Trust Model

- Syllago operates no central registry or marketplace
- Registries are git repos cloned over HTTPS -- integrity is provided by git, not syllago
- **Built-in content** (shipped with the syllago binary) is maintained by the syllago team
- **Registry content** from `syllago registry add <url>` is third-party and unverified
- **App install scripts** from registries require explicit user confirmation before execution
- Users should review hooks and MCP configs before installing -- they execute as your user

## Reporting a Vulnerability

**Email:** openscribbler.dev@pm.me

Subject line: `[SECURITY] syllago -- <brief description>`

**Response targets:**

- Acknowledgment: 48 hours
- Fix or mitigation: 7 days

**Please include:**

- Description of the vulnerability and impact
- Reproduction steps
- Affected versions (check `syllago version`)

This is a pre-revenue open source project. There is no bug bounty program.

## Disclosure Policy

We prefer coordinated disclosure. Please do not open public GitHub issues for security vulnerabilities. We will credit reporters in the security advisory unless you prefer to remain anonymous.
