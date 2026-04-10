---
name: presentation-graphics
description: Create quality graphics for conference presentations using SVG and Mermaid. Supports predefined themes (including RSAC 2026), accessibility compliance, and guided creation workflow.
---

# Presentation Graphics

Create accessible, on-brand graphics for conference presentations.

## Workflow Routing

| Trigger | Workflow | File |
|---------|----------|------|
| "create graphic", "presentation graphic", "slide graphic", "recreate image" | CreateGraphic | `Workflows/CreateGraphic.md` |

---

## CRITICAL: Layout Rules for Full-Slide SVG (1920x1080)

> **STOP**: Before generating ANY full-slide SVG, complete these steps.

### 1. Load Layout Reference

Load `references/layout-calculations.md` FIRST.

### 2. Respect Zone Boundaries

| Zone | Y Range | Rule |
|------|---------|------|
| Title | 0-150 | Title and subtitle only |
| Content | 180-900 | **All panels/diagrams MUST end by y=900** |
| Footer | 920-1080 | Callouts start at y=920 or later |

### 3. Calculate Before Generating

```
For each element:
  y_end = y_start + height
  VERIFY: y_end <= 900 (for content)

For callout:
  callout_y = max(920, max_content_y_end + 20)
```

### 4. Mandatory Spacing

- **Element gaps**: 20px minimum between adjacent elements
- **Text padding**: 16px minimum from container edges
- **Edge margins**: 30px from viewBox edges

**If you skip these calculations, elements WILL overlap.**

---

## Quick Decision Trees

### Format Selection

```
Need to create a graphic
        |
        v
Need icons, custom shapes, precise positioning?
        |-- Yes --> SVG
        |-- No
            v
Need flowcharts, sequences, architecture diagrams?
        |-- Yes --> Mermaid
        |-- No
            v
Recreating an existing image?
        |-- Yes --> SVG
        |-- No --> Consider Mermaid first
```

### Pattern Selection

| Visual Need | Pattern | Recommended Format |
|-------------|---------|-------------------|
| Branded element with icon | Icon+Label | SVG |
| Step-by-step process | Process Flow | SVG or Mermaid |
| Side-by-side comparison | Comparison Grid | SVG |
| Layered system view | Architecture Layers | SVG or Mermaid |
| Chronological events | Timeline | SVG |
| KPI/statistics display | Metrics Cards | SVG |

## Theme Enforcement

**Default behavior**: STRICT - only theme colors allowed

| Flag | Behavior |
|------|----------|
| (none) | Strict enforcement - reject off-theme colors |
| `--allow-custom-colors` | Override - allow any color with warning |

When user requests off-theme colors without flag:
> "Theme enforcement is strict. Use `--allow-custom-colors` to override, or choose from the theme palette."

## Available Themes

| Theme | Use Case | Background | Primary |
|-------|----------|------------|---------|
| RSAC 2026 | RSA Conference presentations | #FFFFFF | #2464C7 |
| Corporate | Generic professional | #FFFFFF | #1976D2 |
| Dark Mode | Dark backgrounds | #121212 | #BB86FC |
| High Contrast | Maximum accessibility | #000000 | #FFFF00 |

Load `references/themes.md` for full color palettes and SVG style blocks.

## Accessibility Quick Check

Before finalizing ANY graphic, verify:

- [ ] **Contrast**: Text meets 4.5:1 ratio (3:1 for 18pt+)
- [ ] **Font size**: Body text 18pt+ equivalent (24px in SVG)
- [ ] **Font family**: Sans-serif only (Calibri, Arial, Arial Nova)
- [ ] **Alt text**: Brief, meaningful, no "image of"
- [ ] **Color independence**: Information not conveyed by color alone

Load `references/accessibility.md` for full requirements and pre-validated color combinations.

### Layout Quick Check
- [ ] Text has adequate padding from container edges (16px+)
- [ ] Elements don't overlap unintentionally
- [ ] Content has margin from viewBox edges (30px+)
- [ ] Labels are concise (2-4 words) - no explanatory text

## When to Load Which Reference

| Task | Load |
|------|------|
| **Full-slide layout (1920x1080)** | [layout-calculations.md](references/layout-calculations.md) - **LOAD FIRST** |
| Creating SVG graphics | [svg-patterns.md](references/svg-patterns.md) |
| Creating Mermaid diagrams | [mermaid-patterns.md](references/mermaid-patterns.md) |
| Selecting/applying themes | [themes.md](references/themes.md) |
| Accessibility validation | [accessibility.md](references/accessibility.md) |
| Interactive creation | [Workflows/CreateGraphic.md](Workflows/CreateGraphic.md) |

## Cross-References

- For architecture diagrams: `skills/architecture-patterns/SKILL.md`
- For complex system design: Collaborate with **senior-architect**
- For documentation context: Collaborate with **senior-technical-writer**
