# Layout Calculations Reference

> **CRITICAL**: Load this reference BEFORE generating any full-slide SVG (1920x1080). Calculate positions first, then generate.

## Canvas Zones (1920x1080 viewBox)

```
y=0    ┌─────────────────────────────────────────────┐
       │           TITLE ZONE (y: 0-150)             │
y=150  ├─────────────────────────────────────────────┤
       │                                             │
       │                                             │
       │         CONTENT ZONE (y: 180-900)           │
       │                                             │
       │                                             │
y=900  ├─────────────────────────────────────────────┤
       │          FOOTER ZONE (y: 920-1080)          │
y=1080 └─────────────────────────────────────────────┘

Horizontal: x=30 to x=1890 (30px edge margins)
```

### Zone Rules

| Zone | Y Range | Purpose | Constraints |
|------|---------|---------|-------------|
| Title | 0-150 | Slide title + subtitle | Title y ≤ 80, subtitle y ≤ 130 |
| Content | 180-900 | Main panels, diagrams | All content y_end ≤ 900 |
| Footer | 920-1080 | Bottom callouts, page numbers | Callout y_start ≥ 920 |
| **Gap** | 900-920 | Buffer between content and footer | NEVER place elements here |

**ABSOLUTE RULE**: Content elements MUST have `y + height ≤ 900`. Callouts MUST have `y ≥ 920`.

---

## Position Calculation Formulas

### Before Placing Any Element

Calculate the element's bounding box:

```
element_y_start = transform translate y + local y
element_y_end = element_y_start + element_height
element_x_start = transform translate x + local x
element_x_end = element_x_start + element_width
```

### Vertical Stacking (Elements Above/Below)

```
Required gap: 20px minimum

element2_y_start >= element1_y_end + 20

Example:
  Panel 1: y=180, height=300 → y_end = 480
  Panel 2: y_start >= 480 + 20 = 500
```

### Bottom Callout Positioning

```
callout_y = max(all_content_y_end) + 20

But ALSO enforce:
  callout_y >= 920 (footer zone minimum)

Final formula:
  callout_y = max(920, max_content_y_end + 20)
```

**Common Pattern - Panel + Callout:**

```
Panel: y=180, height=700 → y_end = 880
Gap: 20px minimum → callout could start at 900
BUT footer zone starts at 920
THEREFORE: callout_y = 920 (or later)
```

---

## Text-in-Container Formulas

### Centering Text Vertically

```
text_y = container_y + (container_height / 2) + (font_size / 3)
```

Example: Container at y=100, height=60, font-size=24
```
text_y = 100 + (60 / 2) + (24 / 3) = 100 + 30 + 8 = 138
```

### Padding Validation

For text to have 16px padding from container edges:

```
Top padding check:
  text_y - font_size >= container_y + 16

Bottom padding check:
  text_y + 4 <= container_y + container_height - 16

Left padding (for left-aligned text):
  text_x >= container_x + 16

Right padding (for right-aligned text):
  text_x <= container_x + container_width - 16
```

### Multi-line Text

For each additional line, add `line_height` (typically font_size × 1.2):

```
line1_y = container_y + padding_top + font_size
line2_y = line1_y + (font_size × 1.2)
line3_y = line2_y + (font_size × 1.2)

Last line must satisfy:
  lineN_y + 4 <= container_y + container_height - 16
```

---

## Collision Detection

### Check Two Elements for Overlap

```
function overlaps(elem1, elem2):
  // Vertical overlap
  vertical_overlap = NOT (elem1.y_end + 20 <= elem2.y_start
                     OR elem2.y_end + 20 <= elem1.y_start)

  // Horizontal overlap
  horizontal_overlap = NOT (elem1.x_end + 20 <= elem2.x_start
                       OR elem2.x_end + 20 <= elem1.x_start)

  return vertical_overlap AND horizontal_overlap
```

### Validate All Elements

Before generating SVG, build a position map and verify:

```
for each pair (elem1, elem2) where elem1 != elem2:
  if overlaps(elem1, elem2):
    ERROR: "Elements overlap - adjust positions"
    SOLUTION: Move elem2.y_start to elem1.y_end + 20
```

---

## Common Layout Patterns

### Two-Column Panel Layout

```
viewBox: 0 0 1920 1080

Left panel:  x=40,  width=900
Right panel: x=980, width=900
Gap between: 980 - (40 + 900) = 40px ✓

Both panels: y=180, height=700
Panel y_end: 880

Bottom callout: y=920, height=120
Callout y_end: 1040 (within 1080) ✓
```

### Panel with Header + Content

```
Panel container: y=180, height=600
  Header bar:    y=180, height=70  (y_end = 250)
  Content area:  y=250, height=530 (y_end = 780)

Panel y_end: 780
Bottom callout: y=920 ✓ (780 + 140 gap)
```

### Stacked Elements Inside Panel

```
Panel: y=180, height=600

Inside panel (relative positions):
  Element 1: y=30, height=80   → absolute y_end = 180+30+80 = 290
  Element 2: y=130, height=80  → absolute y_start = 180+130 = 310
  Gap: 310 - 290 = 20px ✓

  Element 3: y=230, height=80  → absolute y_start = 180+230 = 410
  Gap: 410 - (180+130+80) = 20px ✓
```

---

## Pre-Generation Checklist

Before writing ANY SVG code:

1. **[ ] Define zones**: Title ends at y=150, content ends at y=900, footer starts at y=920
2. **[ ] Calculate panel heights**: Total content must fit in y=180 to y=900 (720px max)
3. **[ ] Position callout**: Find max content y_end, set callout_y = max(920, content_y_end + 20)
4. **[ ] Verify text padding**: All text has 16px+ from container edges
5. **[ ] Check element gaps**: Adjacent elements have 20px+ between them

---

## Anti-Patterns

### WRONG: Callout Overlapping Panel

```xml
<!-- Panel ends at y=180+600=780, but callout starts at y=750 -->
<rect y="180" height="600"/>  <!-- y_end = 780 -->
<rect y="750" height="100"/>  <!-- OVERLAPS! Starts 30px before panel ends -->
```

**Fix**: Move callout to y=920 or reduce panel height.

### WRONG: Text Touching Container Edge

```xml
<rect x="100" y="100" width="200" height="50"/>
<text x="105" y="130">Text</text>  <!-- Only 5px left padding! -->
```

**Fix**: `x="116"` for 16px padding.

### WRONG: No Gap Between Elements

```xml
<rect y="100" height="50"/>  <!-- y_end = 150 -->
<rect y="150" height="50"/>  <!-- y_start = 150, NO GAP! -->
```

**Fix**: Second rect `y="170"` for 20px gap.

---

## Quick Reference Card

```
┌──────────────────────────────────────┐
│         1920x1080 LAYOUT             │
├──────────────────────────────────────┤
│ Title zone:    y = 0 to 150          │
│ Content zone:  y = 180 to 900        │
│ Footer zone:   y = 920 to 1080       │
├──────────────────────────────────────┤
│ Edge margins:  30px all sides        │
│ Element gaps:  20px minimum          │
│ Text padding:  16px from edges       │
├──────────────────────────────────────┤
│ Callout formula:                     │
│   y = max(920, content_y_end + 20)   │
└──────────────────────────────────────┘
```
