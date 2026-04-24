<!-- modeled after: (synthesized) -->
First region. This block contains guidance for setting up a local
development loop. Install the toolchain, bootstrap the workspace, and
run the verification suite to confirm everything is wired up.

The goal of a quick bootstrap is to make it cheap to try changes. Every
minute of friction in the inner loop multiplies over a week of work.

===SYLLAGO-SPLIT===

Second region. This section covers the testing conventions.

Unit tests live alongside the implementation files. Integration tests
sit under tests/integration. End-to-end tests run on a nightly cadence
in CI only.

===SYLLAGO-SPLIT===

Third region. This section covers the release process.

Releases cut from main on a two-week cadence. The release captain tags
the commit and drives promotion through staging to production. Every
release runs on canary before taking full traffic.

===SYLLAGO-SPLIT===

Fourth region. This section covers observability.

Structured logging is the default. Every log line has a level, a
message, and a key-value context map. Traces are added around every
external call that is not already instrumented by the framework.
