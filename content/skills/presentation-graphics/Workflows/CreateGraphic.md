# CreateGraphic Workflow

> **Triggers**: "create graphic", "presentation graphic", "slide graphic", "recreate image", "make a visual"

## Purpose

Interactively guide users through creating presentation graphics with theme compliance and accessibility validation.

## Guided Flow Principles

- **Infer defaults** from context when possible
- **Pause at key decisions** for user confirmation
- **Validate** theme compliance and accessibility before output
- **Provide** alt text recommendation with every graphic

---

## Phase 1: Understand Intent

### 1.1 Identify Graphic Type

Infer from context if possible. If unclear, ask:

> "What type of visual would best communicate this? Options:
> - **Process Flow** - Sequential steps or workflow
> - **Comparison Grid** - Side-by-side comparison
> - **Architecture Layers** - System structure
> - **Timeline** - Chronological events
> - **Metrics Cards** - KPIs or statistics
> - **Icon+Label** - Single branded element
> - **Other** - Describe what you need"

### 1.2 Determine Format

Based on pattern type, suggest format:

| Pattern | Default Format | Rationale |
|---------|---------------|-----------|
| Process Flow (simple) | Mermaid | Easy to modify |
| Process Flow (branded) | SVG | Custom styling |
| Comparison Grid | SVG | Precise layout |
| Architecture Layers | Mermaid or SVG | Depends on complexity |
| Timeline | SVG | Visual precision |
| Metrics Cards | SVG | Custom styling |
| Icon+Label | SVG | Branding control |

Present recommendation:
> "For a [pattern], I recommend [format] because [reason]. Shall I proceed with [format]?"

---

## Phase 2: Collect Content

Based on pattern type, gather required inputs.

**Brevity Principle**: All labels should be 2-4 words maximum. Speakers will elaborate verbally - graphics support the message, they don't contain the full message.

### Process Flow
- Number of steps (3-5 recommended for clarity)
- Step labels (keep brief: 2-4 words each)
- Optional: Step descriptions
- Direction preference (horizontal/vertical)

### Comparison Grid
- Number of columns (2-3 recommended)
- Column headers
- Row items (keep to 3-5 rows)
- Cell values

### Architecture Layers
- Layer names (top to bottom)
- Components per layer (optional)
- Connections between layers (optional)

### Timeline
- Milestone dates or labels
- Event descriptions (brief)
- Number of milestones (4-6 recommended)

### Metrics Cards
- Metric values
- Metric labels
- Number of metrics (1-4 recommended)

### Icon+Label
- Main label text
- Optional subtitle
- Icon description or type

---

## Phase 3: Theme Selection

### 3.1 Present Available Themes

> "Which theme should I use?
> - **RSAC 2026** - RSA Conference official colors (Blue, Lime, Navy)
> - **Corporate** - Professional business (Blue, Teal, Amber)
> - **Dark Mode** - Dark backgrounds (Purple, Teal)
> - **High Contrast** - Maximum accessibility (Yellow on Black)
> - **Custom** - Provide your own colors"

### 3.2 Handle Custom Colors

If user requests custom colors:

**Without `--allow-custom-colors` flag:**
> "Theme enforcement is strict by default. The requested color [X] is not in the [theme] palette.
>
> Options:
> 1. Use the closest theme color: [suggested color]
> 2. Add `--allow-custom-colors` to your request to override
> 3. Choose a different theme"

**With `--allow-custom-colors` flag:**
> "Custom color accepted. Note: Please verify contrast requirements manually.
> - Text on [color]: Ensure 4.5:1 contrast ratio
> - Large text (18pt+): Minimum 3:1 ratio"

### 3.3 Load Theme

Load `references/themes.md` and extract:
- Color palette
- Font specifications
- SVG style block (for SVG output)
- Mermaid theme config (for Mermaid output)

---

## Phase 3.5: Calculate Layout (MANDATORY for Full-Slide SVG)

> **CRITICAL**: For any 1920x1080 SVG, complete this phase BEFORE generating code.

### 3.5.1 Load Layout Reference

Load `references/layout-calculations.md` for zone definitions and formulas.

### 3.5.2 Plan Vertical Zones

Determine what elements will occupy each zone:

| Zone | Y Range | Your Elements |
|------|---------|---------------|
| Title | 0-150 | [title text, subtitle] |
| Content | 180-900 | [panels, diagrams, lists] |
| Footer | 920-1080 | [callout, page number] |

### 3.5.3 Calculate Element Positions

For each content element, calculate:

```
Element: [name]
  y_start: [value]
  height:  [value]
  y_end:   y_start + height = [value]

Verify: y_end <= 900 for content elements
```

For bottom callout:
```
callout_y = max(920, max_content_y_end + 20) = [value]
```

### 3.5.4 Verify No Overlaps

Check each adjacent pair:
```
[Element 1] y_end: [X]
[Element 2] y_start: [Y]
Gap: Y - X = [Z]px
Required: Z >= 20px ✓/✗
```

### 3.5.5 Position Map Output

Document final positions before proceeding:
```
Position Map:
- Title: y=60
- Subtitle: y=120
- Panel 1: y=180, height=350, y_end=530
- Panel 2: y=550, height=330, y_end=880
- Callout: y=920, height=100, y_end=1020
```

**If any overlap detected**: STOP and adjust positions before proceeding to Phase 4.

---

## Phase 4: Generate Graphic

### 4.1 Load Pattern Reference

- SVG patterns: `references/svg-patterns.md`
- Mermaid patterns: `references/mermaid-patterns.md`

### 4.2 Apply Theme

**For SVG:**
1. Insert theme `<defs>` block
2. Apply class names to elements
3. Verify font sizes (24px minimum body, 32px headings)

**For Mermaid:**
1. Insert theme initialization block
2. Apply style classes to nodes
3. Set fontSize in themeVariables

### 4.3 Generate Output

- **Apply positions from Phase 3.5 position map** (do not guess positions)
- Replace all placeholders with user content
- Verify text fits within containers
- Ensure logical reading order

**If Phase 3.5 was skipped for a 1920x1080 SVG**: STOP and complete Phase 3.5 first.

### 4.4 Layout Validation (Computational)

Before proceeding to accessibility checks, **calculate and verify** (not just visually check):

**Zone Compliance:**
```
For each content element:
  Verify: element_y + element_height <= 900
  Result: [element name]: y_end = [value] <= 900 ✓/✗

For callout (if present):
  Verify: callout_y >= 920
  Result: callout_y = [value] >= 920 ✓/✗
```

**Gap Verification:**
```
For each adjacent pair:
  gap = element2_y - element1_y_end
  Verify: gap >= 20
  Result: [elem1] to [elem2]: gap = [value]px >= 20 ✓/✗
```

**Text Padding:**
```
For each text element in a container:
  left_padding = text_x - container_x
  Verify: left_padding >= 16
  Result: [text]: left_padding = [value]px >= 16 ✓/✗
```

**Edge Margins:**
- [ ] Content x_start >= 30
- [ ] Content x_end <= 1890
- [ ] Labels are 2-4 words, not explanations

**If ANY check fails**: Fix the position and re-run validation before continuing.

---

## Phase 5: Accessibility Validation

Before presenting output, run accessibility checks:

### Checklist

- [ ] **Contrast**: All text/background combinations meet requirements
  - Body text: 4.5:1 minimum
  - Large text (18pt+): 3:1 minimum
- [ ] **Font sizes**: Body 24px+, headings 32px+
- [ ] **Font family**: Sans-serif with fallbacks
- [ ] **Color independence**: Information not conveyed by color alone

### Validation Output

Present validation results:

```
Accessibility Check:
- Contrast: [PASS/WARN with details]
- Font Size: [PASS/WARN with details]
- Color Independence: [PASS/WARN with details]
```

If warnings exist, offer to fix:
> "Accessibility warning: [issue]. Would you like me to adjust to [suggested fix]?"

---

## Phase 6: Output & Alt Text

### 6.1 Present Graphic Code

Output the complete graphic (SVG or Mermaid) in a code block.

### 6.2 Provide Alt Text

Generate alt text following guidelines from `references/accessibility.md`:

```
Recommended Alt Text:
"[Pattern-appropriate description following templates]"
```

**Alt text templates by pattern:**

| Pattern | Template |
|---------|----------|
| Process Flow | "Process flow showing [N] steps: [step names]" |
| Comparison Grid | "Comparison of [items]: [key differences]" |
| Architecture | "Architecture diagram with [layers]: [layer names]" |
| Timeline | "Timeline from [start] to [end]: [milestones]" |
| Metrics | "[Metric name]: [value]" |
| Icon+Label | "[Label]: [description]" |

### 6.3 Usage Instructions

```
To use this graphic:
1. Copy the [SVG/Mermaid] code above
2. [Format-specific instructions]
3. Apply the recommended alt text for accessibility

For SVG: Paste into PowerPoint via Insert > Pictures > This Device
For Mermaid: Render using a Mermaid-compatible tool, then export as image
```

---

## Error Handling

| Issue | Response |
|-------|----------|
| Unclear pattern | Present pattern examples with visuals, ask for selection |
| Too many elements | Suggest splitting into multiple graphics |
| Custom colors without flag | Explain enforcement, offer alternatives |
| Low contrast detected | Warn and suggest theme-compliant alternatives |
| Text too long for container | Suggest abbreviations or multi-line |

---

## Quick Reference: Decision Tree

```
User requests graphic
        |
        v
Can pattern be inferred from context?
    |-- Yes --> Confirm pattern with user
    |-- No --> Ask using pattern selection prompt
        |
        v
Is format preference stated?
    |-- Yes --> Use stated format
    |-- No --> Recommend based on pattern
        |
        v
Collect pattern-specific content
        |
        v
Is theme specified?
    |-- Yes --> Load theme
    |-- No --> Present theme options
        |
        v
Custom colors requested?
    |-- No --> Proceed
    |-- Yes --> Check for override flag
        |
        v
Generate graphic
        |
        v
Run accessibility validation
        |
        v
Present graphic + alt text + usage instructions
```
