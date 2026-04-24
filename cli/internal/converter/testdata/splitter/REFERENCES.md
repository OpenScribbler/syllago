# Splitter fixture references

These fixtures are synthesized (content fabricated). Their structural shape
is modeled after the real-world references below for coverage traceability.
No third-party content is committed.

## CLAUDE.md / AGENTS.md / GEMINI.md shaped

| Fixture | Modeled after | Structural shape captured |
|---|---|---|
| h2-clean.md | saaspegasus/pegasus-docs CLAUDE.md | ~45L, 7 H2s, no preamble |
| h2-with-preamble.md | steadycursor/steadystart CLAUDE.md | ~142L, 9 H2s, 4-line preamble |
| h2-numbered-prefix.md | nammayatri/nammayatri .clinerules | ## 1., ## 2. patterns |
| h2-emoji-prefix.md | grahama1970/claude-code-mcp-enhanced CLAUDE.md | emoji-prefixed headings, slug normalization |
| h3-deep.md | kubernetes/kops AGENTS.md | 118L, 3 H2 / 11 H3 |
| h4-rare.md | payloadcms/payload CLAUDE.md | H4 splitting case |
| marker-literal.md | (synthesized) | literal custom-marker shape |
| too-small.md | victrme/Bonjourr AGENTS.md | <30L skip-split trigger |
| no-h2.md | (synthesized) | 0 H2s skip-split trigger |
| delegating-stub.md | pathintegral-institute/mcpm.sh GEMINI.md | 1-line delegation |
| table-heavy.md | DataDog/lading AGENTS.md | tables-in-content stress |
| decorative-hr.md | p33m5t3r/vibecoding/conway CLAUDE.md | standalone --- as decoration |
| must-should-may.md | pingcap/tidb AGENTS.md | mandate-language casing preservation |
| trailing-whitespace.md | (synthesized for D12) | two trailing spaces on a line |
| crlf-line-endings.md | (synthesized for D12) | CRLF throughout |
| bom-prefix.md | (synthesized for D12) | leading UTF-8 BOM |
| no-trailing-newline.md | (synthesized for D12) | missing final newline |
| import-line.md | (synthesized for D4) | @import line preservation |

## .cursorrules shaped

| Fixture | Modeled after | Structural shape captured |
|---|---|---|
| cursorrules-flat-numbered.md | level09/enferno | numbered flat list, anti-fixture for "don't split" |
| cursorrules-points-elsewhere.md | uhop/stream-json | medium, points to AGENTS.md |

## .clinerules shaped

| Fixture | Modeled after | Structural shape captured |
|---|---|---|
| clinerules-numbered-h2.md | nammayatri/nammayatri | 105L, numbered ## N. Topic H2s |

## .windsurfrules shaped

| Fixture | Modeled after | Structural shape captured |
|---|---|---|
| windsurfrules-pointer.md | SAP/fundamental-ngx | 1-line pointer — nonsensical to split |
| windsurfrules-numbered-rules.md | level09/enferno | 17 numbered rules |
