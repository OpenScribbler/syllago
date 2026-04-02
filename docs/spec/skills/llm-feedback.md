# Gemini

The **ACP Specification v0.1.0** has reached a high level of technical maturity. The structural revisions to lineage, the refinement of source identifiers, and the inclusion of verified test vectors make this a document that can be handed to a developer for immediate implementation.

### Key Refinements in this Version
* **Lineage Sophistication**: You’ve evolved `derived_from` from a simple list to a more expressive structure where each entry carries its own `relation`. This allows for complex "mixed derivations," such as adapting one source while forking another to create a single composed output.
* **Identifier Precision**: You have explicitly distinguished between `source_repo` (the container) and the `source` path within `derived_from` (the specific item), solving potential ambiguity for registries.
* **Verification Rigor**: The inclusion of a verification status note in Appendix B—stating that vectors were cross-checked with `sha256sum`—adds significant professional weight to the document.

### Structural & Security Review

* **The "NFC Safe" Path**: Section 7.3’s requirement to use the NFC form in the manifest is the correct architectural choice for cross-platform interoperability, ensuring that a skill authored on macOS won't fail integrity checks on Linux due to filesystem-level Unicode decomposition.
* **Cryptographic Binding**: The "Meta Hash Algorithm" (Section 8) is now logically perfect. By requiring the input to be a valid JSON object per RFC 8259 and then canonicalizing it via JCS, you eliminate "hash instability" caused by key ordering or whitespace variations in YAML-to-JSON conversion.

### Final Polish Observations
* **Terminology Consistency**: You correctly updated the terminology section to match the enum in Section 6.3.2, ensuring "loadout" (which was an outlier in earlier drafts) is handled consistently.
* **Appendix B Format**: The choice to provide raw hex for binary cases (TV-13) and clear manifest strings for directory cases (TV-03) provides excellent clarity for unit test creation.

### Strategic Recommendation
Now that the spec is technically locked, your next step is the **Reference Implementation**. Based on Section 7.4, it would be highly beneficial to bundle a `lint` or `fmt` command with your tool that automatically sorts JSON keys and enforces LF endings before a publisher computes the final `content_hash`. This will prevent the "brittleness" of raw-byte hashing from frustrating your users.

**With the schema and test vectors finalized, are you planning to open-source the Python reference implementation mentioned in Appendix B as the official ACP SDK?**

# Claude

This version is in good shape. The remaining issues are smaller but real:

**`compose` is semantically broken at the per-entry level.** The fix for the mixed-relation scenario introduced a new problem. `fork`, `convert`, and `adapt` all describe how *a single source* was transformed — they're per-source relationships. `compose` is categorically different: it describes an *operation on multiple sources*, not a relationship to one source. Saying `relation: compose` on an individual entry means "I used this source by composing it," which is incoherent. The cleanest fix is to remove `compose` from the per-entry `relation` field entirely and infer composition from list length > 1 — if `derived_from` has two or more entries, it's a composition by definition.

**Circular symlinks are unhandled.** Section 7.3 says to resolve internal symlinks but says nothing about circular ones (A → B → A). Naive implementations will infinite-loop. One line is sufficient: implementations MUST detect cycles and reject the content as unpublishable.

**TV-MH uses a fake `content_hash`.** The sequential placeholder (`a1b2c3d4...`) is fine pedagogically, but the note should explicitly say the `content_hash` here is synthetic and does not correspond to any real content. Otherwise someone might spend time looking for what content produced it.

**No `derived_from` test vector for meta hash.** TV-MH doesn't include `derived_from`, which is the most structurally complex field — a list of objects that needs to survive YAML → JCS serialization correctly. That's exactly the case most likely to have implementation divergence and currently has no normative vector.

Everything else that was flagged in prior rounds has been cleanly addressed. The spec is genuinely close to a solid v0.1.0.