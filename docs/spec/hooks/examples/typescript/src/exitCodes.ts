/**
 * exitCodes.ts — Exit Code Resolution
 * Reference implementation: Extended conformance (§5, §8.2)
 *
 * Covers:
 *   §4    Exit Code Contract — exit code semantics and normalization
 *   §5.2  Interaction with Exit Codes — precedence rules
 *   §5.3  Evaluation Order — non-blocking downgrade then truth table lookup
 */

// ---------------------------------------------------------------------------
// Type definitions
// ---------------------------------------------------------------------------

/** The JSON `decision` field from §5.1. `null` represents an absent field. */
export type Decision = "allow" | "deny" | "ask";

/** The resolved result after applying the full evaluation order from §5.3. */
export type Result = "allow" | "block" | "warn_allow" | "ask";

// ---------------------------------------------------------------------------
// Resolution
// ---------------------------------------------------------------------------

/**
 * Resolve the combined exit code and JSON decision into a final result.
 *
 * Implements the evaluation order defined in §5.3:
 *
 * 1. **Normalize exit code** — Any code other than 0, 1, or 2 is treated as 1
 *    (§4: "Other" exit codes have the same behavior as exit code 1).
 *
 * 2. **Non-blocking downgrade** — If `blocking` is `false` and the normalized
 *    exit code is 2, it is downgraded to 1 before further evaluation (§5.3 step 1).
 *
 * 3. **Truth table lookup** — The normalized (and possibly downgraded) exit code
 *    is combined with `decision` to produce the final result (§5.3 step 2).
 *
 * @param blocking  Whether the hook definition has `blocking: true`.
 * @param exitCode  The raw integer exit code from the hook process.
 * @param decision  The parsed `decision` field from stdout JSON, or `null` when
 *                  absent (empty stdout, `{}`, or no `decision` key).
 */
export function resolve(
  blocking: boolean,
  exitCode: number,
  decision: Decision | null,
): Result {
  // Step 1 — Normalize: map any exit code outside {0, 1, 2} to 1.
  const normalizedCode = exitCode === 0 || exitCode === 2 ? exitCode : 1;

  // Step 2 — Non-blocking downgrade: exit code 2 becomes 1 when blocking is false.
  const effectiveCode =
    !blocking && normalizedCode === 2 ? 1 : normalizedCode;

  // Step 3 — Truth table lookup (§5.3).
  if (effectiveCode === 1) {
    // exit code 1 always yields warn_allow, regardless of blocking or decision.
    return "warn_allow";
  }

  // effectiveCode is 0 from here on. Blocking only matters for exit code 2,
  // which is handled above; for code 0 the decision field drives the result.
  if (effectiveCode === 2) {
    // Only reachable when blocking === true (downgrade already applied above).
    // exit code 2 overrides decision: "allow" per §5.2.
    return "block";
  }

  // effectiveCode === 0: decision field is authoritative.
  switch (decision) {
    case "deny":
      // decision: "deny" with exit code 0 is treated as a block (§5.2).
      return "block";
    case "ask":
      // decision: "ask" defers to the user (§5.2). The caller is responsible
      // for degrading to "block" in non-interactive environments.
      return "ask";
    case "allow":
    case null:
      // decision: "allow" or absent — normal success path (§5.2).
      return "allow";
  }
}
