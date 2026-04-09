# Presentation Themes

Complete theme definitions with color palettes and SVG style blocks.

## RSAC 2026

RSA Conference 2026 official theme. Use for all RSAC presentations.

### Color Palette

| Role | Color | Hex | Usage |
|------|-------|-----|-------|
| Background | White | #FFFFFF | Slide backgrounds |
| Text | Black | #000000 | Body text |
| Primary Accent | Blue | #2464C7 | Headers, key elements, links |
| Secondary Accent | Lime Green | #A6CE38 | Success states, highlights |
| Tertiary Accent | Dark Navy | #051464 | Strong emphasis, dark headers |
| Highlight 1 | Magenta/Pink | #D823AD | Call-to-action, attention |
| Highlight 2 | Teal | #23ABAD | Secondary CTAs, followed links |
| Highlight 3 | Orange | #D36B37 | Warnings, alerts |
| Highlight 4 | Sky Blue | #74C1EC | Light backgrounds, fills |
| Muted | Light Blue-Gray | #CCD8EA | Subtle backgrounds, borders |

### Fonts

| Usage | Font | Fallback |
|-------|------|----------|
| Headings | Arial Nova | Arial |
| Body | Arial Nova | Arial |
| Captions | Arial Nova | Arial |

### Recommended Combinations (Pre-Validated for Accessibility)

| Use Case | Background | Foreground | Accent | Contrast |
|----------|------------|------------|--------|----------|
| Standard slide | #FFFFFF | #000000 | #2464C7 | 21:1 |
| Emphasis section | #051464 | #FFFFFF | #A6CE38 | 14.7:1 |
| Call-to-action | #FFFFFF | #051464 | #D823AD | 14.7:1 |
| Data/metrics | #FFFFFF | #000000 | #23ABAD | 21:1 |
| Warning state | #FFFFFF | #000000 | #D36B37 | 21:1 |

### SVG Theme Block

```xml
<!-- RSAC 2026 Theme -->
<defs>
  <style>
    .rsac-bg { fill: #FFFFFF; }
    .rsac-text { fill: #000000; font-family: Calibri, Arial, sans-serif; }
    .rsac-heading { fill: #051464; font-family: Calibri, Arial, sans-serif; font-weight: bold; }
    .rsac-primary { fill: #2464C7; }
    .rsac-secondary { fill: #A6CE38; }
    .rsac-tertiary { fill: #051464; }
    .rsac-highlight { fill: #D823AD; }
    .rsac-highlight2 { fill: #23ABAD; }
    .rsac-highlight3 { fill: #D36B37; }
    .rsac-highlight4 { fill: #74C1EC; }
    .rsac-muted { fill: #CCD8EA; }
    .rsac-text-on-dark { fill: #FFFFFF; font-family: Calibri, Arial, sans-serif; }
    .rsac-text-on-primary { fill: #FFFFFF; font-family: Calibri, Arial, sans-serif; }
  </style>
</defs>
```

---

## Corporate

Generic professional theme for business presentations.

### Color Palette

| Role | Color | Hex | Usage |
|------|-------|-----|-------|
| Background | White | #FFFFFF | Slide backgrounds |
| Text | Dark Gray | #212121 | Body text |
| Primary Accent | Blue | #1976D2 | Headers, key elements |
| Secondary Accent | Teal | #00897B | Success, secondary actions |
| Tertiary Accent | Amber | #FFA000 | Highlights, warnings |
| Muted | Light Gray | #F5F5F5 | Subtle backgrounds |

### Fonts

| Usage | Font | Fallback |
|-------|------|----------|
| Headings | Arial | Helvetica, sans-serif |
| Body | Arial | Helvetica, sans-serif |

### SVG Theme Block

```xml
<!-- Corporate Theme -->
<defs>
  <style>
    .corp-bg { fill: #FFFFFF; }
    .corp-text { fill: #212121; font-family: Arial, Helvetica, sans-serif; }
    .corp-heading { fill: #1976D2; font-family: Arial, Helvetica, sans-serif; font-weight: bold; }
    .corp-primary { fill: #1976D2; }
    .corp-secondary { fill: #00897B; }
    .corp-tertiary { fill: #FFA000; }
    .corp-muted { fill: #F5F5F5; }
    .corp-text-on-primary { fill: #FFFFFF; font-family: Arial, Helvetica, sans-serif; }
  </style>
</defs>
```

---

## Dark Mode

Dark background theme for modern presentations.

### Color Palette

| Role | Color | Hex | Usage |
|------|-------|-----|-------|
| Background | Dark | #121212 | Slide backgrounds |
| Surface | Elevated | #1E1E1E | Cards, containers |
| Text | White | #FFFFFF | Body text |
| Primary Accent | Purple | #BB86FC | Headers, key elements |
| Secondary Accent | Teal | #03DAC6 | Success, secondary actions |
| Error | Red | #CF6679 | Errors, warnings |

### SVG Theme Block

```xml
<!-- Dark Mode Theme -->
<defs>
  <style>
    .dark-bg { fill: #121212; }
    .dark-surface { fill: #1E1E1E; }
    .dark-text { fill: #FFFFFF; font-family: Arial, Helvetica, sans-serif; }
    .dark-heading { fill: #BB86FC; font-family: Arial, Helvetica, sans-serif; font-weight: bold; }
    .dark-primary { fill: #BB86FC; }
    .dark-secondary { fill: #03DAC6; }
    .dark-error { fill: #CF6679; }
    .dark-text-on-primary { fill: #000000; font-family: Arial, Helvetica, sans-serif; }
  </style>
</defs>
```

---

## High Contrast

Maximum accessibility theme for visually impaired audiences.

### Color Palette

| Role | Color | Hex | Usage |
|------|-------|-----|-------|
| Background | Black | #000000 | Slide backgrounds |
| Text | White | #FFFFFF | Body text |
| Primary Accent | Yellow | #FFFF00 | Headers, key elements |
| Secondary Accent | Cyan | #00FFFF | Links, secondary actions |
| Alert | Magenta | #FF00FF | Warnings, attention |

### SVG Theme Block

```xml
<!-- High Contrast Theme -->
<defs>
  <style>
    .hc-bg { fill: #000000; }
    .hc-text { fill: #FFFFFF; font-family: Arial, Helvetica, sans-serif; }
    .hc-heading { fill: #FFFF00; font-family: Arial, Helvetica, sans-serif; font-weight: bold; }
    .hc-primary { fill: #FFFF00; }
    .hc-secondary { fill: #00FFFF; }
    .hc-alert { fill: #FF00FF; }
  </style>
</defs>
```

---

## Inline Theme Definition

For custom themes, define inline using this format:

```yaml
name: Custom Theme
colors:
  background: "#HEXCODE"
  text: "#HEXCODE"
  primary: "#HEXCODE"
  secondary: "#HEXCODE"
  accent: "#HEXCODE"
fonts:
  heading: "Font Name"
  body: "Font Name"
  fallback: "sans-serif"
```

When using inline themes:
1. Validate contrast ratios against accessibility requirements
2. Use only sans-serif fonts
3. Include fallback fonts
4. Test all color combinations used

## Theme Application Checklist

Before applying any theme:

- [ ] Verify all text/background combinations meet contrast requirements
- [ ] Confirm fonts are sans-serif with fallbacks
- [ ] Check accent colors work on both light and dark text
- [ ] Test any custom combinations against `references/accessibility.md`
