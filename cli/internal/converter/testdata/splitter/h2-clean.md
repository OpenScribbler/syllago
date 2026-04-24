<!-- modeled after: saaspegasus/pegasus-docs CLAUDE.md -->
## Setup Instructions

Install dependencies with the package manager of your choice. The repository
supports both uv and pip workflows for Python, with yarn recommended for the
frontend assets. Use the version pinned in the lockfile.

Run the bootstrap script once after cloning. It prepares environment files,
applies migrations against the local database, and seeds the example dataset
that powers the onboarding tour.

## Project Structure

The backend lives under apps/api and the frontend under apps/web. Shared
utilities sit in packages/shared and are imported by both. Migrations are
tracked in apps/api/migrations with timestamp-prefixed filenames.

Static assets are compiled into dist/ and served by the web tier in production.
Never edit files under dist/ by hand.

## Testing

Run the full suite with make test. Individual packages expose their own
make targets for quick iteration. Integration tests require a running
database; use make db-up to start one locally.

Unit tests are expected to pass in under ten seconds. If a unit test takes
longer than that, it is probably an integration test in disguise and should
be moved.

## Deployment

Production deployments flow through the main branch. Tag a release with the
semantic version before promoting. The staging environment tracks develop
automatically, so merge there first for acceptance testing.

## Contributing

Small focused pull requests land faster. Prefer several small PRs over one
large one. Link the issue in the description and keep the branch up to date
with main.
