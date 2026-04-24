<!-- modeled after: level09/enferno .cursorrules -->
1. Use the package manager pinned in the lockfile. Do not upgrade
   dependencies in unrelated PRs.
2. Write tests alongside the code they cover. Table-driven style.
3. Commit messages use Conventional Commits. Type and scope required.
4. Lint and format on every save. The editor config is the source of
   truth for whitespace.
5. Never commit secrets. Use the vault integration for production
   credentials and .env.example for local defaults.
6. Prefer explicit names over clever ones. Code is read more often
   than it is written.
7. Pull requests are small and focused. Several small PRs land faster
   than one large one.
8. Every PR needs at least one reviewer from a different sub-team.
9. Self-merge is allowed only for documentation fixes.
10. Review latency target is one business day.
11. Releases cut on the first Tuesday of each month.
12. Every release passes through canary before full promotion.
13. Rollback is a one-command operation; prior artifacts stay hot for
    twenty-four hours after rollout.
14. Structured logging everywhere. Every log line has a level, a
    message, and a context map.
15. Metrics and traces are opt-in at the call site but enforced at the
    boundary by middleware.
16. Error handling propagates errors up to a boundary with enough
    context to decide what to do.
17. Do not swallow errors in empty catch or except blocks.
18. Hot paths are benchmarked. Regressions over five percent block
    merge unless documented.
19. Memory pools require a comment explaining why pooling was chosen.
20. Write for the next reader. Comments answer why, not what.
