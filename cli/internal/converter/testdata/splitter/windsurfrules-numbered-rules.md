<!-- modeled after: level09/enferno .windsurfrules -->
1. Use the pinned toolchain version. Never upgrade dependencies in
   unrelated PRs.
2. Tests live alongside the code they cover. Table-driven style is
   the default.
3. Conventional Commits required. Scope the type to the subsystem.
4. Lint and format on every save. The editor config enforces the
   whitespace rules automatically.
5. Never commit secrets. Use the vault integration for production
   credentials.
6. Prefer explicit names over clever ones. Code is read more often
   than it is written.
7. PRs are small and focused. Several small PRs land faster than
   one large one.
8. Every PR needs a reviewer from another sub-team. Review latency
   target is one business day.
9. Self-merge is allowed only for documentation fixes.
10. Releases cut on the first Tuesday of each month.
11. Canary always precedes promotion. Every release takes canary
    traffic for at least one hour.
12. Rollback is a one-command operation. Prior artifacts stay hot
    for twenty-four hours after rollout.
13. Structured logging everywhere. Every log line has a level, a
    message, and a context map.
14. Propagate errors with context. Do not swallow errors in empty
    catch or except blocks.
15. Hot paths are benchmarked. Regressions over five percent block
    merge unless documented.
16. Memory pools require a comment explaining why pooling was
    chosen over a fresh allocation.
17. Write for the next reader. Comments answer why, not what.
