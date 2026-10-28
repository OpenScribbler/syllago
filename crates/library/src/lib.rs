//! Syllago's Library and sync module. So far it holds the signed loadout
//! manifest: its grammar, its digest, the loadout message an Author Key
//! signs, and the decision sync makes for a loadout from its signature and
//! this machine's record of it ([`loadout`]).
//!
//! The crate reads bytes and returns facts. Finding the manifests, keeping
//! the records and applying the loadouts belong to its caller.

pub mod loadout;
