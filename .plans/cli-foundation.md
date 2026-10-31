# Feature: CLI foundation
Status: IN_PROGRESS
Issue: #4
Branch: feat/4-cli-foundation
PR: #11
Claim-ID: a4d128b9-8127-437c-a2d9-692619ee1e15

## Goal
Replace the draft with a reusable, tested CLI foundation.
## Requirements and acceptance criteria
Help, version, shared flags, errors, cancellation, separate results and rendering, documented output contract.
## In scope
Standard-library command runner, JSON envelope, text renderer, signal context and deterministic tests.
## Out of scope
Network diagnostic implementations, release artifacts.
## Dependencies and constraints
No runtime dependencies. Linux AMD64 and ARM64. Continue the existing session claim.
## Decisions and open questions
Separate the runner into internal/cli. Command functions return structured data; text rendering is a separate function. Common flags can appear before or after the command. Execution timeout defaults to 10s. JSON errors remain on stderr with empty stdout. Use uppercase constant names per dkstack.
## Commit plan
### C01: Replace draft foundation
Status: IN_PROGRESS
Depends on: none
Proposed commit: feat(cli): replace draft command foundation
Actual commit: none
Objective: Make the command contract usable by the diagnostic issues.
Work: Write failing behavior tests, implement runner and renderers, document contract.
Verification: Tests, vet, formatting, race test, Linux builds and binary smoke checks.
Evidence: New runner tests first failed with undefined Run and Command. Tests now pass, including race checks. Vet, formatting, diff checks, AMD64 and ARM64 builds pass. A deliberate timeout mutation caused TestDiagnosticRendering to fail and was restored. Independent design review findings addressed or documented.
## Final verification
Inspect the diff against issue #4 and run all project gates.
## Handoff
Next action: Commit and update PR #11.
Blockers: none
Risks: Command-specific result schemas will be defined by the diagnostic issues.
