# Security Policy

## Scope

Vulnerabilities we want to hear about:

- **Malicious content injection** -- rules, skills, agents, or hooks that execute attacker-controlled code through syllago's install or apply mechanisms
- **Path traversal** -- add, install, or export operations writing files outside the expected target directory
- **MCP config injection** -- JSON merge inserting unexpected server entries into provider settings
- **Symlink escape** -- content that resolves outside the expected install target via symlink manipulation
- **Registry trust bypass** -- mechanisms to install unconfirmed content without user awareness or confirmation
- **Private content leakage** -- content from private registries being published or shared to public registries through syllago commands

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

## Registry Privacy

Syllago includes a privacy gate system to prevent accidental leakage of content from private registries to public destinations.

- **Detection:** Private registries are identified via hosting platform APIs (GitHub, GitLab, Bitbucket) and an optional `visibility` field in `registry.yaml`. Unknown visibility defaults to private.
- **Tainting:** Content imported from private registries is permanently tagged with its source registry and visibility in metadata. This taint persists through the content's lifecycle in the library.
- **Enforcement:** Four gates block private-tainted content from reaching public registries -- at `publish`, `share`, `loadout create` (warning), and `loadout publish` (block).
- **Scope of protection:** This is a soft gate designed to prevent accidental leakage through syllago commands. It does not prevent intentional circumvention via direct filesystem operations, direct git commands, or modification of content after export. There is no override flag by design -- removing the taint requires re-adding the content from a public source.

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
