# 0012. --provider validation against LoadAllSourceManifests vs. provider.AllProviders

Date: 2026-05-07
Status: Deprecated
Feature: capmon-fetch-subcommand

## Context

The capmon pipeline (Stage 1) discovers providers solely from `docs/provider-sources/*.yaml` (`pipeline.go:150`). Only 9 of the 15 entries in `provider.AllProviders` have Provider Source Manifests. If the command validated against `AllProviders` and the user requested a slug with no manifest (e.g., `amp`, `codex`), the validation would pass but the fetch loop would produce zero output — a silent no-op that is harder to diagnose than a hard error. Validating against the set of manifest slugs produces an informative error message that lists fetchable providers.

## Decision

Chose **Validate against slugs derived from `LoadAllSourceManifests` (`cli/internal/capmon/sourceman.go:127`)** over **Validate against `provider.AllProviders` (`cli/internal/provider/provider.go:86`)**.

## Consequences

The error message for an unknown slug must be constructed at runtime from `LoadAllSourceManifests`, not from a static string. This means `LoadAllSourceManifests` is called once for validation and once in the fetch loop; a small redundancy but acceptable because manifest loading is a cheap filesystem scan of 9 YAML files.
