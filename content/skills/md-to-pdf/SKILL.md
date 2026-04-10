---
name: md-to-pdf
description: |
  Convert markdown files with Mermaid diagrams to high-quality PDF.
  Trigger: /md-to-pdf <filepath>
---

# Markdown to PDF Conversion

Convert markdown files to PDF with high-quality Mermaid diagram rendering.

## Quick Reference

```
/md-to-pdf <filepath>
```

| Parameter | Description |
|-----------|-------------|
| `filepath` | Path to markdown file (absolute or relative) |

**Output:** PDF file in same directory as input (e.g., `doc.md` -> `doc.pdf`)

## Prerequisites

Check and install required tools before conversion:

```bash
# Check if tools are installed
command -v mmdc >/dev/null 2>&1 || echo "mermaid-cli not installed"
command -v md-to-pdf >/dev/null 2>&1 || echo "md-to-pdf not installed"
```

| Tool | Package | Install Command |
|------|---------|-----------------|
| `mmdc` | @mermaid-js/mermaid-cli | `npm install -g @mermaid-js/mermaid-cli` |
| `md-to-pdf` | md-to-pdf | `npm install -g md-to-pdf` |

If prerequisites are missing, report to user with install commands and stop.

## Conversion Workflow

Execute these steps in order:

### Step 1: Pre-render Mermaid Diagrams

```bash
mmdc -i <input>.md -o <input>-rendered.md -e png -s 3
```

| Flag | Purpose |
|------|---------|
| `-i` | Input markdown file |
| `-o` | Output rendered markdown (with embedded PNGs) |
| `-e png` | Export diagrams as PNG images |
| `-s 3` | Scale factor 3x for high quality |

**Note:** mmdc gracefully handles markdown without mermaid blocks (passes through unchanged).

### Step 2: Convert to PDF

```bash
md-to-pdf <input>-rendered.md --md-file-encoding utf-8
```

This creates `<input>-rendered.pdf` in the same directory.

### Step 3: Rename and Cleanup

```bash
mv <input>-rendered.pdf <input>.pdf
rm <input>-rendered.md
rm <input>-rendered-*.png 2>/dev/null || true
```

## Progress Output

Report progress at each step:

```
Converting <filename>.md to PDF...
  [1/3] Pre-rendering Mermaid diagrams at 3x scale...
  [2/3] Converting markdown to PDF...
  [3/3] Cleaning up temporary files...
Done: <filename>.pdf
```

## Error Handling

| Error | Cause | Action |
|-------|-------|--------|
| `mmdc: command not found` | mermaid-cli not installed | Report: `npm install -g @mermaid-js/mermaid-cli` |
| `md-to-pdf: command not found` | md-to-pdf not installed | Report: `npm install -g md-to-pdf` |
| `File not found` | Invalid input path | Verify file exists, check path |
| `mmdc` fails | Invalid mermaid syntax | Show mmdc error output, check diagram syntax |
| `md-to-pdf` fails | Markdown conversion error | Show md-to-pdf error output |
| Cleanup fails | Temp files missing | Ignore (use `|| true`) |
