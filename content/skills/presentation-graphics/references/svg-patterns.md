# SVG Pattern Templates

SVG templates for each presentation graphic pattern. Copy and customize these templates, applying theme styles from `themes.md`.

> **CRITICAL for Full-Slide SVG (1920x1080)**: Load `layout-calculations.md` FIRST and calculate element positions BEFORE generating code. Templates below use small viewBoxes - full-slide layouts require zone-aware positioning.

## Pattern: Icon+Label

Single branded element with icon and text. Use for feature callouts, key points, or branded elements.

### Basic Template

```xml
<svg viewBox="0 0 300 100" xmlns="http://www.w3.org/2000/svg">
  <!-- Insert theme <defs> block here -->

  <!-- Icon placeholder (50x50) -->
  <rect x="20" y="25" width="50" height="50" class="rsac-primary" rx="8"/>

  <!-- Label (18pt+ equivalent) -->
  <text x="90" y="45" class="rsac-heading" font-size="24">{{LABEL}}</text>

  <!-- Optional subtitle -->
  <text x="90" y="70" class="rsac-text" font-size="18">{{SUBTITLE}}</text>
</svg>
```

### With Circle Icon

```xml
<svg viewBox="0 0 300 100" xmlns="http://www.w3.org/2000/svg">
  <!-- Theme defs -->

  <!-- Circular icon background -->
  <circle cx="45" cy="50" r="30" class="rsac-primary"/>

  <!-- Icon placeholder (replace with actual icon path) -->
  <text x="45" y="58" text-anchor="middle" class="rsac-text-on-primary" font-size="24">?</text>

  <!-- Label -->
  <text x="95" y="45" class="rsac-heading" font-size="24">{{LABEL}}</text>
  <text x="95" y="70" class="rsac-text" font-size="18">{{SUBTITLE}}</text>
</svg>
```

### Customization Points
- `{{LABEL}}`: Main text (24px minimum)
- `{{SUBTITLE}}`: Supporting text (18px minimum)
- Icon: Replace placeholder with SVG path or symbol

---

## Pattern: Process Flow

Sequential steps showing progression. Use for workflows, procedures, or multi-step concepts.

### Horizontal 3-Step

```xml
<svg viewBox="0 0 700 140" xmlns="http://www.w3.org/2000/svg">
  <!-- Theme defs -->

  <!-- Step 1 -->
  <rect x="20" y="40" width="180" height="60" class="rsac-primary" rx="8"/>
  <text x="110" y="75" text-anchor="middle" class="rsac-text-on-primary" font-size="20">{{STEP_1}}</text>

  <!-- Arrow 1 -->
  <path d="M 210 70 L 250 70" stroke="#000000" fill="none" stroke-width="3"/>
  <polygon points="250,70 240,60 240,80" fill="#000000"/>

  <!-- Step 2 -->
  <rect x="260" y="40" width="180" height="60" class="rsac-secondary" rx="8"/>
  <text x="350" y="75" text-anchor="middle" class="rsac-text" font-size="20">{{STEP_2}}</text>

  <!-- Arrow 2 -->
  <path d="M 450 70 L 490 70" stroke="#000000" fill="none" stroke-width="3"/>
  <polygon points="490,70 480,60 480,80" fill="#000000"/>

  <!-- Step 3 -->
  <rect x="500" y="40" width="180" height="60" class="rsac-tertiary" rx="8"/>
  <text x="590" y="75" text-anchor="middle" class="rsac-text-on-dark" font-size="20">{{STEP_3}}</text>
</svg>
```

### Horizontal 4-Step

```xml
<svg viewBox="0 0 900 140" xmlns="http://www.w3.org/2000/svg">
  <!-- Theme defs -->

  <!-- Step 1 -->
  <rect x="20" y="40" width="160" height="60" class="rsac-primary" rx="8"/>
  <text x="100" y="75" text-anchor="middle" class="rsac-text-on-primary" font-size="18">{{STEP_1}}</text>

  <!-- Arrow 1 -->
  <path d="M 190 70 L 220 70" stroke="#000" fill="none" stroke-width="2"/>
  <polygon points="220,70 212,64 212,76" fill="#000"/>

  <!-- Step 2 -->
  <rect x="230" y="40" width="160" height="60" class="rsac-secondary" rx="8"/>
  <text x="310" y="75" text-anchor="middle" class="rsac-text" font-size="18">{{STEP_2}}</text>

  <!-- Arrow 2 -->
  <path d="M 400 70 L 430 70" stroke="#000" fill="none" stroke-width="2"/>
  <polygon points="430,70 422,64 422,76" fill="#000"/>

  <!-- Step 3 -->
  <rect x="440" y="40" width="160" height="60" class="rsac-highlight2" rx="8"/>
  <text x="520" y="75" text-anchor="middle" class="rsac-text" font-size="18">{{STEP_3}}</text>

  <!-- Arrow 3 -->
  <path d="M 610 70 L 640 70" stroke="#000" fill="none" stroke-width="2"/>
  <polygon points="640,70 632,64 632,76" fill="#000"/>

  <!-- Step 4 -->
  <rect x="650" y="40" width="160" height="60" class="rsac-tertiary" rx="8"/>
  <text x="730" y="75" text-anchor="middle" class="rsac-text-on-dark" font-size="18">{{STEP_4}}</text>
</svg>
```

### Vertical 3-Step

```xml
<svg viewBox="0 0 300 400" xmlns="http://www.w3.org/2000/svg">
  <!-- Theme defs -->

  <!-- Step 1 -->
  <rect x="50" y="20" width="200" height="80" class="rsac-primary" rx="8"/>
  <text x="150" y="55" text-anchor="middle" class="rsac-text-on-primary" font-size="20">{{STEP_1}}</text>
  <text x="150" y="80" text-anchor="middle" class="rsac-text-on-primary" font-size="14">{{DESC_1}}</text>

  <!-- Arrow 1 -->
  <path d="M 150 100 L 150 130" stroke="#000" fill="none" stroke-width="2"/>
  <polygon points="150,140 144,130 156,130" fill="#000"/>

  <!-- Step 2 -->
  <rect x="50" y="150" width="200" height="80" class="rsac-secondary" rx="8"/>
  <text x="150" y="185" text-anchor="middle" class="rsac-text" font-size="20">{{STEP_2}}</text>
  <text x="150" y="210" text-anchor="middle" class="rsac-text" font-size="14">{{DESC_2}}</text>

  <!-- Arrow 2 -->
  <path d="M 150 230 L 150 260" stroke="#000" fill="none" stroke-width="2"/>
  <polygon points="150,270 144,260 156,260" fill="#000"/>

  <!-- Step 3 -->
  <rect x="50" y="280" width="200" height="80" class="rsac-tertiary" rx="8"/>
  <text x="150" y="315" text-anchor="middle" class="rsac-text-on-dark" font-size="20">{{STEP_3}}</text>
  <text x="150" y="340" text-anchor="middle" class="rsac-text-on-dark" font-size="14">{{DESC_3}}</text>
</svg>
```

---

## Pattern: Comparison Grid

Side-by-side comparison of options, features, or products.

### 2-Column Comparison

```xml
<svg viewBox="0 0 600 300" xmlns="http://www.w3.org/2000/svg">
  <!-- Theme defs -->

  <!-- Headers -->
  <rect x="20" y="20" width="270" height="50" class="rsac-primary" rx="8"/>
  <text x="155" y="52" text-anchor="middle" class="rsac-text-on-primary" font-size="22">{{OPTION_A}}</text>

  <rect x="310" y="20" width="270" height="50" class="rsac-secondary" rx="8"/>
  <text x="445" y="52" text-anchor="middle" class="rsac-text" font-size="22">{{OPTION_B}}</text>

  <!-- Row 1 -->
  <rect x="20" y="80" width="270" height="40" class="rsac-muted" rx="4"/>
  <text x="30" y="105" class="rsac-text" font-size="18">{{ROW_1_A}}</text>

  <rect x="310" y="80" width="270" height="40" class="rsac-muted" rx="4"/>
  <text x="320" y="105" class="rsac-text" font-size="18">{{ROW_1_B}}</text>

  <!-- Row 2 -->
  <rect x="20" y="130" width="270" height="40" fill="#FFFFFF" stroke="#CCD8EA" rx="4"/>
  <text x="30" y="155" class="rsac-text" font-size="18">{{ROW_2_A}}</text>

  <rect x="310" y="130" width="270" height="40" fill="#FFFFFF" stroke="#CCD8EA" rx="4"/>
  <text x="320" y="155" class="rsac-text" font-size="18">{{ROW_2_B}}</text>

  <!-- Row 3 -->
  <rect x="20" y="180" width="270" height="40" class="rsac-muted" rx="4"/>
  <text x="30" y="205" class="rsac-text" font-size="18">{{ROW_3_A}}</text>

  <rect x="310" y="180" width="270" height="40" class="rsac-muted" rx="4"/>
  <text x="320" y="205" class="rsac-text" font-size="18">{{ROW_3_B}}</text>
</svg>
```

### 3-Column Feature Grid

```xml
<svg viewBox="0 0 800 350" xmlns="http://www.w3.org/2000/svg">
  <!-- Theme defs -->

  <!-- Column Headers -->
  <rect x="20" y="20" width="240" height="50" class="rsac-primary" rx="8"/>
  <text x="140" y="52" text-anchor="middle" class="rsac-text-on-primary" font-size="20">{{COL_1}}</text>

  <rect x="280" y="20" width="240" height="50" class="rsac-secondary" rx="8"/>
  <text x="400" y="52" text-anchor="middle" class="rsac-text" font-size="20">{{COL_2}}</text>

  <rect x="540" y="20" width="240" height="50" class="rsac-highlight2" rx="8"/>
  <text x="660" y="52" text-anchor="middle" class="rsac-text" font-size="20">{{COL_3}}</text>

  <!-- Feature rows (repeat pattern) -->
  <!-- Row 1 -->
  <text x="140" y="100" text-anchor="middle" class="rsac-text" font-size="18">{{R1_C1}}</text>
  <text x="400" y="100" text-anchor="middle" class="rsac-text" font-size="18">{{R1_C2}}</text>
  <text x="660" y="100" text-anchor="middle" class="rsac-text" font-size="18">{{R1_C3}}</text>
  <line x1="20" y1="115" x2="780" y2="115" stroke="#CCD8EA" stroke-width="1"/>

  <!-- Row 2 -->
  <text x="140" y="145" text-anchor="middle" class="rsac-text" font-size="18">{{R2_C1}}</text>
  <text x="400" y="145" text-anchor="middle" class="rsac-text" font-size="18">{{R2_C2}}</text>
  <text x="660" y="145" text-anchor="middle" class="rsac-text" font-size="18">{{R2_C3}}</text>
  <line x1="20" y1="160" x2="780" y2="160" stroke="#CCD8EA" stroke-width="1"/>
</svg>
```

---

## Pattern: Architecture Layers

Stacked layers showing system architecture.

### 3-Layer Stack

```xml
<svg viewBox="0 0 500 350" xmlns="http://www.w3.org/2000/svg">
  <!-- Theme defs -->

  <!-- Layer 1 (Top) -->
  <rect x="50" y="30" width="400" height="80" class="rsac-primary" rx="8"/>
  <text x="250" y="60" text-anchor="middle" class="rsac-text-on-primary" font-size="22">{{LAYER_1}}</text>
  <text x="250" y="90" text-anchor="middle" class="rsac-text-on-primary" font-size="16">{{LAYER_1_DESC}}</text>

  <!-- Connector 1 -->
  <path d="M 250 110 L 250 130" stroke="#000" stroke-width="2"/>
  <polygon points="250,140 244,130 256,130" fill="#000"/>

  <!-- Layer 2 (Middle) -->
  <rect x="50" y="150" width="400" height="80" class="rsac-secondary" rx="8"/>
  <text x="250" y="180" text-anchor="middle" class="rsac-text" font-size="22">{{LAYER_2}}</text>
  <text x="250" y="210" text-anchor="middle" class="rsac-text" font-size="16">{{LAYER_2_DESC}}</text>

  <!-- Connector 2 -->
  <path d="M 250 230 L 250 250" stroke="#000" stroke-width="2"/>
  <polygon points="250,260 244,250 256,250" fill="#000"/>

  <!-- Layer 3 (Bottom) -->
  <rect x="50" y="270" width="400" height="80" class="rsac-tertiary" rx="8"/>
  <text x="250" y="300" text-anchor="middle" class="rsac-text-on-dark" font-size="22">{{LAYER_3}}</text>
  <text x="250" y="330" text-anchor="middle" class="rsac-text-on-dark" font-size="16">{{LAYER_3_DESC}}</text>
</svg>
```

### Layer with Components

```xml
<svg viewBox="0 0 600 200" xmlns="http://www.w3.org/2000/svg">
  <!-- Theme defs -->

  <!-- Layer background -->
  <rect x="20" y="20" width="560" height="160" class="rsac-muted" rx="8" stroke="#051464" stroke-width="2"/>
  <text x="40" y="50" class="rsac-heading" font-size="20">{{LAYER_NAME}}</text>

  <!-- Components inside layer -->
  <rect x="40" y="70" width="150" height="90" class="rsac-primary" rx="4"/>
  <text x="115" y="120" text-anchor="middle" class="rsac-text-on-primary" font-size="16">{{COMP_1}}</text>

  <rect x="220" y="70" width="150" height="90" class="rsac-primary" rx="4"/>
  <text x="295" y="120" text-anchor="middle" class="rsac-text-on-primary" font-size="16">{{COMP_2}}</text>

  <rect x="400" y="70" width="150" height="90" class="rsac-primary" rx="4"/>
  <text x="475" y="120" text-anchor="middle" class="rsac-text-on-primary" font-size="16">{{COMP_3}}</text>
</svg>
```

---

## Pattern: Timeline

Chronological events or milestones.

### Horizontal Timeline

```xml
<svg viewBox="0 0 800 180" xmlns="http://www.w3.org/2000/svg">
  <!-- Theme defs -->

  <!-- Timeline line -->
  <line x1="50" y1="90" x2="750" y2="90" stroke="#CCD8EA" stroke-width="4"/>

  <!-- Milestone 1 -->
  <circle cx="100" cy="90" r="15" class="rsac-primary"/>
  <text x="100" y="95" text-anchor="middle" class="rsac-text-on-primary" font-size="14">1</text>
  <text x="100" y="130" text-anchor="middle" class="rsac-heading" font-size="16">{{DATE_1}}</text>
  <text x="100" y="155" text-anchor="middle" class="rsac-text" font-size="14">{{EVENT_1}}</text>

  <!-- Milestone 2 -->
  <circle cx="300" cy="90" r="15" class="rsac-secondary"/>
  <text x="300" y="95" text-anchor="middle" class="rsac-text" font-size="14">2</text>
  <text x="300" y="50" text-anchor="middle" class="rsac-heading" font-size="16">{{DATE_2}}</text>
  <text x="300" y="70" text-anchor="middle" class="rsac-text" font-size="14">{{EVENT_2}}</text>

  <!-- Milestone 3 -->
  <circle cx="500" cy="90" r="15" class="rsac-highlight2"/>
  <text x="500" y="95" text-anchor="middle" class="rsac-text" font-size="14">3</text>
  <text x="500" y="130" text-anchor="middle" class="rsac-heading" font-size="16">{{DATE_3}}</text>
  <text x="500" y="155" text-anchor="middle" class="rsac-text" font-size="14">{{EVENT_3}}</text>

  <!-- Milestone 4 -->
  <circle cx="700" cy="90" r="15" class="rsac-tertiary"/>
  <text x="700" y="95" text-anchor="middle" class="rsac-text-on-dark" font-size="14">4</text>
  <text x="700" y="50" text-anchor="middle" class="rsac-heading" font-size="16">{{DATE_4}}</text>
  <text x="700" y="70" text-anchor="middle" class="rsac-text" font-size="14">{{EVENT_4}}</text>
</svg>
```

---

## Pattern: Metrics Cards

Large numbers with labels for KPIs and statistics.

### Single Metric Card

```xml
<svg viewBox="0 0 200 150" xmlns="http://www.w3.org/2000/svg">
  <!-- Theme defs -->

  <!-- Card background -->
  <rect x="10" y="10" width="180" height="130" fill="#FFFFFF" stroke="#CCD8EA" stroke-width="2" rx="8"/>

  <!-- Large metric value -->
  <text x="100" y="75" text-anchor="middle" class="rsac-primary" font-size="48" font-weight="bold">{{VALUE}}</text>

  <!-- Label -->
  <text x="100" y="110" text-anchor="middle" class="rsac-text" font-size="16">{{LABEL}}</text>
</svg>
```

### 3-Metric Dashboard

```xml
<svg viewBox="0 0 700 180" xmlns="http://www.w3.org/2000/svg">
  <!-- Theme defs -->

  <!-- Card 1 -->
  <rect x="20" y="20" width="200" height="140" fill="#FFFFFF" stroke="#2464C7" stroke-width="2" rx="8"/>
  <text x="120" y="85" text-anchor="middle" class="rsac-primary" font-size="42" font-weight="bold">{{VAL_1}}</text>
  <text x="120" y="125" text-anchor="middle" class="rsac-text" font-size="16">{{LABEL_1}}</text>

  <!-- Card 2 -->
  <rect x="250" y="20" width="200" height="140" fill="#FFFFFF" stroke="#A6CE38" stroke-width="2" rx="8"/>
  <text x="350" y="85" text-anchor="middle" class="rsac-secondary" font-size="42" font-weight="bold">{{VAL_2}}</text>
  <text x="350" y="125" text-anchor="middle" class="rsac-text" font-size="16">{{LABEL_2}}</text>

  <!-- Card 3 -->
  <rect x="480" y="20" width="200" height="140" fill="#FFFFFF" stroke="#23ABAD" stroke-width="2" rx="8"/>
  <text x="580" y="85" text-anchor="middle" class="rsac-highlight2" font-size="42" font-weight="bold">{{VAL_3}}</text>
  <text x="580" y="125" text-anchor="middle" class="rsac-text" font-size="16">{{LABEL_3}}</text>
</svg>
```

---

## Common SVG Elements

### Arrows

```xml
<!-- Right arrow -->
<path d="M 0 10 L 30 10" stroke="currentColor" fill="none" stroke-width="2"/>
<polygon points="30,10 22,5 22,15" fill="currentColor"/>

<!-- Down arrow -->
<path d="M 10 0 L 10 30" stroke="currentColor" fill="none" stroke-width="2"/>
<polygon points="10,30 5,22 15,22" fill="currentColor"/>

<!-- Bidirectional arrow -->
<path d="M 10 15 L 90 15" stroke="currentColor" fill="none" stroke-width="2"/>
<polygon points="10,15 18,10 18,20" fill="currentColor"/>
<polygon points="90,15 82,10 82,20" fill="currentColor"/>
```

### Badges/Pills

```xml
<!-- Numbered badge -->
<circle cx="25" cy="25" r="20" class="rsac-primary"/>
<text x="25" y="32" text-anchor="middle" class="rsac-text-on-primary" font-size="18">1</text>

<!-- Status pill -->
<rect x="0" y="0" width="80" height="30" class="rsac-secondary" rx="15"/>
<text x="40" y="22" text-anchor="middle" class="rsac-text" font-size="14">Active</text>
```

### Icons (Placeholder Shapes)

```xml
<!-- Checkmark placeholder -->
<circle cx="20" cy="20" r="18" class="rsac-secondary"/>
<path d="M 10 20 L 17 27 L 30 13" stroke="#FFFFFF" fill="none" stroke-width="3"/>

<!-- Warning placeholder -->
<polygon points="20,5 35,35 5,35" class="rsac-highlight3"/>
<text x="20" y="30" text-anchor="middle" fill="#FFFFFF" font-size="18" font-weight="bold">!</text>
```

## Layout Rules

Follow these rules to prevent common SVG layout issues:

### Padding Requirements
- **Text in containers**: Minimum 16px padding from text baseline to container edge
- **Content boxes**: 20px minimum internal padding on all sides
- **Nested elements**: Each nesting level adds 10px padding

### Element Spacing
- **Adjacent elements**: Minimum 20px gap between neighboring elements
- **Diagram components**: 30px minimum between distinct visual groups
- **Collision avoidance**: Before placing an element, verify it doesn't overlap existing elements

### ViewBox Margins
- **Edge buffer**: 30px minimum from content to viewBox edges
- **Safe zone**: Keep critical content within 90% of viewBox dimensions

### Text Brevity
- **Labels**: 2-4 words maximum - speakers will elaborate
- **Headers**: 3-6 words maximum
- **Bullet points**: Single phrase, not full sentences
- **Avoid**: Explanatory text, definitions, or descriptions that belong in speaker notes

### Layout Validation Checklist
Before finalizing any SVG:
- [ ] All text has 16px+ padding from container edges
- [ ] No elements overlap unintentionally
- [ ] 20px+ gaps between adjacent elements
- [ ] Content stays 30px from viewBox edges
- [ ] Labels are 2-4 words, not explanations

### Full-Slide Layout (1920x1080 viewBox)

> **See `layout-calculations.md` for complete formulas and collision detection.**

When creating presentation slides with standard 1920x1080 viewBox:

**Vertical Zone Boundaries (ABSOLUTE RULES):**

| Zone | Y Range | Constraint |
|------|---------|------------|
| Title | 0-150 | Title text only |
| Content | 180-900 | **All content y_end MUST be ≤ 900** |
| Buffer | 900-920 | NO elements allowed |
| Footer | 920-1080 | Callouts start at y ≥ 920 |

**Before Generating - Calculate Position Map:**
```
For each element:
  y_end = y_start + height
  CHECK: y_end <= 900 (content) or y_start >= 920 (callout)

For callout:
  callout_y = max(920, max_content_y_end + 20)
```

**Full-Slide Template Shell:**
```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1920 1080">
  <!-- ZONE: Title (y: 0-150) -->
  <text x="960" y="80" text-anchor="middle">Title Here</text>

  <!-- ZONE: Content (y: 180-900) - ALL content y_end <= 900 -->
  <g transform="translate(40, 180)">
    <!-- Panels, diagrams go here -->
    <!-- Calculate: 180 + element heights <= 900 -->
  </g>

  <!-- ZONE: Footer (y: 920-1080) -->
  <g transform="translate(40, 920)">
    <!-- Callouts go here - NEVER above y=920 -->
  </g>
</svg>
```

**Common Mistake - Overlapping Bottom Callouts:**
```
WRONG: Content panel ends at y=940, callout at y=900 (overlaps by 40px)
RIGHT: Content panel height reduced so y_end=880, callout at y=920 (40px clearance)
```

**Verify rounded corners are visible:**
- Container borders with rx/ry (rounded corners) need clearance
- If a callout covers the bottom of a panel, the rounded corners won't render
- Always verify all four corners of content containers are visible

---

## Template Customization Checklist

Before using any template:

- [ ] Insert appropriate theme `<defs>` block
- [ ] Replace all `{{PLACEHOLDER}}` values
- [ ] Verify font sizes meet accessibility minimums (24px body, 32px headings)
- [ ] Check text fits within containers
- [ ] Validate color combinations against `accessibility.md`
