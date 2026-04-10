# Accessibility Requirements

Based on Microsoft PowerPoint accessibility guidelines for presentations.

Reference: [Make your PowerPoint presentations accessible](https://support.microsoft.com/en-us/office/make-your-powerpoint-presentations-accessible-to-people-with-disabilities-6f7772b2-2f33-4bd2-8ca7-dae3b2b3ef25)

## Contrast Requirements

### Minimum Contrast Ratios (WCAG 2.1 AA)

| Element | Minimum Ratio | Standard |
|---------|---------------|----------|
| Body text (< 18pt) | 4.5:1 | WCAG AA |
| Large text (18pt+ / 14pt bold) | 3:1 | WCAG AA |
| Graphical elements, icons | 3:1 | WCAG AA |
| Enhanced (AAA) | 7:1 | WCAG AAA |

### Pre-Validated Combinations (RSAC 2026)

**PASS - Safe to use:**

| Background | Foreground | Ratio | Notes |
|------------|------------|-------|-------|
| #FFFFFF | #000000 | 21:1 | Maximum contrast |
| #FFFFFF | #051464 | 14.7:1 | Navy on white |
| #051464 | #FFFFFF | 14.7:1 | White on navy |
| #2464C7 | #FFFFFF | 4.6:1 | White on blue |
| #A6CE38 | #000000 | 9.3:1 | Black on lime |
| #23ABAD | #000000 | 5.8:1 | Black on teal |
| #D36B37 | #000000 | 4.9:1 | Black on orange |
| #D823AD | #FFFFFF | 3.2:1 | Large text only |

**FAIL - Avoid these combinations:**

| Background | Foreground | Ratio | Issue |
|------------|------------|-------|-------|
| #CCD8EA | #FFFFFF | 1.3:1 | Insufficient contrast |
| #74C1EC | #FFFFFF | 1.9:1 | Insufficient contrast |
| #A6CE38 | #FFFFFF | 1.5:1 | Insufficient contrast |
| #CCD8EA | #74C1EC | 1.4:1 | Insufficient contrast |

### Contrast Checking

For custom color combinations, calculate contrast ratio:

```
Contrast Ratio = (L1 + 0.05) / (L2 + 0.05)
```

Where L1 is the relative luminance of the lighter color and L2 is the darker.

Online tools: [WebAIM Contrast Checker](https://webaim.org/resources/contrastchecker/)

## Typography Requirements

### Font Sizes

| Element | Minimum | Recommended | SVG Equivalent |
|---------|---------|-------------|----------------|
| Body text | 18pt | 24pt | 24-32px |
| Headings | 24pt | 32pt+ | 32-42px |
| Labels | 14pt | 18pt | 18-24px |
| Captions | 12pt | 14pt | 16-18px |

### Font Families

**ALLOWED (sans-serif only):**
- Calibri
- Arial
- Arial Nova
- Helvetica
- Verdana
- Tahoma

**AVOID:**
- Serif fonts (Times New Roman, Georgia)
- Decorative/script fonts
- Narrow/condensed fonts
- Light weight fonts (< 400)

### Text Formatting

- **Alignment**: Left-align body text (never justify)
- **Line spacing**: 1.15 to 1.5 line height
- **Letter spacing**: Normal or slightly increased
- **Paragraph spacing**: 1.5x font size minimum

## Alt Text Guidelines

### DO

- Be brief and descriptive (125 characters ideal, 250 max)
- Describe the **purpose/meaning**, not just appearance
- Include any text shown in the graphic
- Start with content type if helpful ("Chart showing...")
- Describe data trends, not every data point

### DON'T

- Start with "Image of" or "Picture of"
- Include redundant information already in surrounding text
- Use file names as alt text
- Leave alt text empty (unless purely decorative)
- Describe every visual detail

### Alt Text Templates by Pattern

| Pattern | Template |
|---------|----------|
| Icon+Label | "[Label]: [what it represents]" |
| Process Flow | "Process: [step names in order]" |
| Comparison Grid | "Comparison of [items]: [key differences]" |
| Architecture Layers | "Architecture: [layers top to bottom]" |
| Timeline | "Timeline from [start] to [end]: [key milestones]" |
| Metrics Cards | "[Metric name]: [value]" |

### Examples

**Good:**
> "Authentication flow: User submits credentials, server validates against IdP, token returned to client"

**Bad:**
> "Image of a diagram showing three blue boxes with arrows"

## Reading Order

For complex graphics:

1. Order elements logically (top-to-bottom, left-to-right)
2. Group related elements together
3. Number steps in process flows
4. Use consistent flow direction

## Color Independence

**Never convey information by color alone.**

For each use of color to indicate meaning, add:
- Text labels
- Patterns or shapes
- Icons or symbols

**Example - Status indicators:**
- Instead of: Red/Yellow/Green circles
- Use: Red X / Yellow ! / Green ✓ with labels

## Accessibility Checklist

Run before finalizing ANY presentation graphic:

### Contrast
- [ ] All body text meets 4.5:1 ratio
- [ ] All large text (18pt+) meets 3:1 ratio
- [ ] All graphical elements meet 3:1 ratio
- [ ] No text on problematic backgrounds (see FAIL list)

### Typography
- [ ] Body text 18pt+ equivalent (24px SVG)
- [ ] Sans-serif fonts only
- [ ] Left-aligned text
- [ ] Adequate line/letter spacing

### Alt Text
- [ ] Alt text prepared for every graphic
- [ ] Alt text is brief and meaningful
- [ ] Does not start with "image of"
- [ ] Includes any text shown in graphic

### Color Independence
- [ ] Information not conveyed by color alone
- [ ] Patterns/labels supplement color coding

### Structure
- [ ] Reading order is logical
- [ ] Related elements are grouped
- [ ] Steps are numbered/labeled

## Quick Validation Script

When generating SVG, verify these minimums:

```
Font sizes: >= 24px for body, >= 32px for headings
Font family: must include sans-serif fallback
Text contrast: check all text/background pairs
```
