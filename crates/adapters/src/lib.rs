//! Provider adapters. An adapter declares what is particular to one
//! provider: its tier, the tool version its golden fixtures were checked
//! against, and its cross-reads, the other providers' locations it also
//! loads content from. It holds behavior only. Paths come from the capmon
//! feed's install-target rows and formats from the `acif` crate, so
//! [`place`] is the same code for every provider.
//!
//! [`place`] decides where one item is written for a set of targeted
//! providers so that each of them loads it exactly once: when one location
//! is read by every targeted provider, such as a shared `AGENTS.md`, the item
//! is written there once; otherwise each provider gets its own write target,
//! and a provider that would still load the item from two locations is
//! refused.

use std::fmt;

use acif::{
    Body, Diagnostic, EntryPoint, Item, Kind, Os, Rendering, Scope, Target, render_rule_for,
    render_skill, resolve,
};

/// How far a provider's paths are trusted.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Tier {
    /// Paths verified against a real install, with golden fixtures.
    Stable,
    /// Paths not yet confirmed against a real install; an apply warns.
    Experimental,
}

/// A location outside a provider's own rows that it also loads: the write
/// target of `provider`'s rows for `kind` at `scope`. It names the other
/// provider rather than a path, so the path still comes from the feed.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct CrossRead {
    pub provider: &'static str,
    pub kind: Kind,
    pub scope: Scope,
}

/// One provider's adapter.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Adapter {
    /// capmon's provider slug.
    pub slug: &'static str,
    pub tier: Tier,
    /// The tool version the golden fixtures were checked against.
    pub tool_version: &'static str,
    pub cross_reads: &'static [CrossRead],
}

/// Claude Code. Its rows name every location it reads skills and rules
/// from; it does not read `~/.agents/skills` or `AGENTS.md` at user scope,
/// so it declares no cross-reads.
pub const CLAUDE_CODE: Adapter = Adapter {
    slug: "claude-code",
    tier: Tier::Stable,
    tool_version: "2.1.312",
    cross_reads: &[],
};

/// Codex CLI. `~/.agents/skills` and `AGENTS.md` are its own rows, so it
/// declares no cross-reads.
pub const CODEX: Adapter = Adapter {
    slug: "codex",
    tier: Tier::Stable,
    tool_version: "0.162.1",
    cross_reads: &[],
};

/// Every adapter syllago ships.
pub const ADAPTERS: [&Adapter; 2] = [&CLAUDE_CODE, &CODEX];

/// The providers whose rows placing for `adapters` reads: theirs and those
/// their cross-reads name. Load the feed for these.
pub fn feed_providers(adapters: &[&Adapter]) -> Vec<&'static str> {
    let mut slugs = Vec::new();
    let named = adapters
        .iter()
        .flat_map(|a| std::iter::once(a.slug).chain(a.cross_reads.iter().map(|c| c.provider)));
    for slug in named {
        if !slugs.contains(&slug) {
            slugs.push(slug);
        }
    }
    slugs
}

/// Install-target rows by provider and content type.
pub trait InstallRecords {
    /// `None` when the records hold nothing for `provider` and `kind`.
    fn entry_points(&self, provider: &str, kind: Kind) -> Option<&[EntryPoint]>;
}

impl InstallRecords for feed::Feed {
    fn entry_points(&self, provider: &str, kind: Kind) -> Option<&[EntryPoint]> {
        feed::Feed::entry_points(self, provider, kind)
    }
}

/// An item to place: its ingested form, and the body and entry file it was
/// ingested from.
#[derive(Debug, Clone, Copy)]
pub struct Source<'a> {
    pub item: &'a Item,
    pub body: &'a Body,
    pub entry_file: &'a str,
}

/// Where an item is written, which targeted providers load it there, and
/// what is written.
#[derive(Debug, Clone, PartialEq)]
pub struct Placement {
    pub target: Target,
    pub providers: Vec<&'static str>,
    pub rendering: Rendering,
    /// Resolution and render diagnostics, such as a superseded write row or
    /// a rule whose activation degraded.
    pub diagnostics: Vec<Diagnostic>,
}

/// Why an item could not be placed.
#[derive(Debug)]
pub enum PlaceError {
    /// The records hold no rows for this provider and content type.
    NotInFeed { provider: &'static str, kind: Kind },
    /// Resolving a provider's rows was refused (`acif.install.*`).
    Resolve {
        provider: &'static str,
        error: acif::Error,
    },
    /// The item could not be rendered for a target.
    Render { path: String, error: acif::Error },
    /// A provider would load the item from more than one location.
    LoadedTwice {
        provider: &'static str,
        locations: Vec<String>,
    },
}

impl PlaceError {
    pub fn id(&self) -> &'static str {
        match self {
            PlaceError::NotInFeed { .. } => "syllago.adapter.not_in_feed",
            PlaceError::Resolve { error, .. } | PlaceError::Render { error, .. } => match error {
                acif::Error::Reject(id) => id,
                acif::Error::Refuse(d) | acif::Error::Nonconformant(d) => d.id,
                _ => "syllago.adapter.unsupported",
            },
            PlaceError::LoadedTwice { .. } => "syllago.adapter.loaded_twice",
        }
    }
}

impl fmt::Display for PlaceError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let id = self.id();
        match self {
            PlaceError::NotInFeed { provider, kind } => write!(
                f,
                "{id}: the feed holds no {} rows for {provider}",
                kind.as_str()
            ),
            PlaceError::Resolve { provider, error } => {
                write!(f, "{id}: resolving {provider}'s location: {error}")
            }
            PlaceError::Render { path, error } => write!(f, "{id}: rendering for {path}: {error}"),
            PlaceError::LoadedTwice {
                provider,
                locations,
            } => write!(
                f,
                "{id}: {provider} would load the item from {}; target fewer providers together",
                locations.join(" and ")
            ),
        }
    }
}

impl std::error::Error for PlaceError {}

/// Places `source` as `name` at `scope` on `os` for every provider in
/// `adapters`, so each loads it exactly once. Only skills and rules are
/// placed.
pub fn place(
    adapters: &[&Adapter],
    records: &impl InstallRecords,
    source: &Source<'_>,
    name: &str,
    scope: Scope,
    os: Os,
) -> Result<Vec<Placement>, PlaceError> {
    let kind = source.item.kind;
    let resolve_for = |provider: &'static str| {
        let rows = records
            .entry_points(provider, kind)
            .ok_or(PlaceError::NotInFeed { provider, kind })?;
        resolve(rows, scope, os, name).map_err(|error| PlaceError::Resolve { provider, error })
    };

    // Each targeted provider's write target and every location it loads.
    let mut resolved = Vec::new();
    for adapter in adapters {
        let resolution = resolve_for(adapter.slug)?;
        let mut reads: Vec<String> = resolution
            .reads
            .iter()
            .map(|t| t.location().to_string())
            .collect();
        for cross in adapter.cross_reads {
            if cross.kind == kind && cross.scope == scope {
                reads.push(resolve_for(cross.provider)?.write.location().to_string());
            }
        }
        resolved.push((adapter.slug, resolution, reads));
    }
    let readers = |location: &str| -> Vec<&'static str> {
        resolved
            .iter()
            .filter(|(_, _, reads)| reads.iter().any(|r| r == location))
            .map(|(slug, _, _)| *slug)
            .collect()
    };

    // Candidates are the write targets, widest-read first; a candidate is
    // taken when none of its readers is already covered.
    let mut candidates: Vec<_> = Vec::new();
    for (_, resolution, _) in &resolved {
        let location = resolution.write.location();
        if !candidates.iter().any(|(_, l, _)| *l == location) {
            candidates.push((resolution, location, readers(location)));
        }
    }
    candidates.sort_by_key(|(_, _, readers)| std::cmp::Reverse(readers.len()));
    let mut covered: Vec<&'static str> = Vec::new();
    let mut chosen = Vec::new();
    for (resolution, location, readers) in candidates {
        if readers.iter().any(|r| covered.contains(r)) {
            continue;
        }
        covered.extend(&readers);
        chosen.push((resolution, location, readers));
    }
    if let Some((slug, resolution, _)) = resolved.iter().find(|(s, _, _)| !covered.contains(s)) {
        // Its own target was skipped because a provider already covered
        // also reads it, so that provider would load the item twice.
        let location = resolution.write.location();
        let provider = readers(location)
            .into_iter()
            .find(|r| covered.contains(r))
            .unwrap_or(slug);
        let first = chosen
            .iter()
            .find(|(_, _, readers)| readers.contains(&provider))
            .map_or(location, |(_, l, _)| *l);
        return Err(PlaceError::LoadedTwice {
            provider,
            locations: vec![first.to_string(), location.to_string()],
        });
    }

    chosen
        .into_iter()
        .map(|(resolution, _, providers)| {
            let target = resolution.write.clone();
            let rendered = match kind {
                Kind::Skill => render_skill(source.body, source.entry_file, target.layout),
                Kind::Rule => render_rule_for(&source.item.canonical, target.layout),
                other => Err(acif::Error::Unsupported(format!(
                    "placing a {} item",
                    other.as_str()
                ))),
            }
            .map_err(|error| PlaceError::Render {
                path: target.path.clone(),
                error,
            })?;
            let mut diagnostics = resolution.diagnostics.clone();
            diagnostics.extend(rendered.diagnostics);
            Ok(Placement {
                target,
                providers,
                rendering: rendered.rendering,
                diagnostics,
            })
        })
        .collect()
}
