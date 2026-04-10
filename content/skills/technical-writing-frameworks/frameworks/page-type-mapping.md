# Page Type to 7-Action Model Mapping

Each page type focuses on 1-2 specific actions. Attempting all seven creates unfocused documentation.

## Page Type Mappings

### Use Cases and Overview Pages
**Actions**: Onboard + Adopt (decision-making)
**Diataxis**: Explanation
**Include**: Value proposition, problem/solution framing, use cases, strategic benefits, "who is this for"
**Exclude**: Step-by-step implementation, API references, troubleshooting, production code examples

### Tutorials
**Actions**: Adopt + Use (basic)
**Diataxis**: Tutorial
**Include**: Learning objectives, prerequisites, one-path steps, expected outcomes, success confirmation, "what's next"
**Exclude**: Multiple approaches, deep explanations, production patterns, comprehensive error handling, all config options

### How-To Guides
**Primary**: Use | **Secondary**: Adopt, Administer, Optimize
**Diataxis**: How-To Guide
**Include**: Clear goal in title, prerequisites with links, steps, multiple approaches, code/CLI examples, troubleshooting
**Exclude**: Strategic value props, beginner hand-holding, every config option, deep background

### Reference Documentation
**Primary**: Administer, Optimize | **Secondary**: Use (lookup), Troubleshoot (error codes)
**Diataxis**: Reference
**Include**: Complete option/parameter lists, types, constraints, defaults, brief examples, cross-references
**Exclude**: Step-by-step instructions, learning content, concept explanations, long task flows

### Troubleshooting Pages
**Action**: Troubleshoot
**Diataxis**: How-To + Reference
**Include**: Error messages, diagnostic flowcharts, symptom-cause-solution mapping, log guidance, when to contact support
**Exclude**: Strategic overview, normal operation instructions, complete API specs, getting-started content

## Decision Framework

1. **Identify page type** (Use Case, Tutorial, How-To, Reference, Troubleshooting)
2. **Check primary actions**: Does the page focus on 1-2 primary actions for that type?
3. **Evaluate secondary actions**: Supporting or distracting? If distracting, consider splitting
4. **Check for excluded actions**: Minor inclusion = add wayfinding; major = split or restructure

## Common Mismatches

- **Tutorial as Reference**: Config tables mid-flow overwhelm beginners. Keep minimal, link to Reference
- **Use Case with implementation**: Strategic readers and developers both frustrated. Keep strategic, create separate How-To
- **How-To with strategic overview**: 3 paragraphs of "why" before task. Max 1-2 sentences context, link to Use Case
- **Reference with instructions**: "First, initialize the client..." breaks scannable format. Move to How-To

## Wayfinding Between Page Types

- Use Case -> Tutorial: "Ready to get started? See [Tutorial]"
- Tutorial -> How-To: "Now learn to [advanced task]"
- How-To -> Reference: "See [Reference] for all options"
- How-To -> Troubleshooting: "Having issues? See [Troubleshooting]"
- Reference -> How-To: "For usage examples, see [How-To]"

## Summary

| Page Type | Primary Actions | Diataxis |
|-----------|----------------|----------|
| Use Case | Onboard, Adopt | Explanation |
| Tutorial | Adopt, Use (basic) | Tutorial |
| How-To | Use | How-To Guide |
| Reference | Administer, Optimize | Reference |
| Troubleshooting | Troubleshoot | How-To + Reference |
