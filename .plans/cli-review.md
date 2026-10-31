# Feature: CLI review corrections
Status: IN_PROGRESS
Issue: #4
Branch: feat/4-cli-foundation
PR: #11
Claim-ID: a4d128b9-8127-437c-a2d9-692619ee1e15

## Goal
Address review S1 and S2.
## Requirements and acceptance criteria
Command-owned options must not require edits to the shared parser. Define version shortcut semantics.
## In scope
Invocation-local flag registration, version validation, tests and documentation.
## Out of scope
Diagnostic implementations.
## Dependencies and constraints
Use standard-library flag parsing and retain the output contract.
## Decisions and open questions
Command.New registers flags and returns an invocation-local execution closure. Diagnostics remain independent of flag.FlagSet. Common flags remain reserved. Version flags are standalone shortcuts and reject another command.
## Commit plan
### C01: Address review
Status: IN_PROGRESS
Depends on: none
Proposed commit: fix(cli): support command-owned flags
Actual commit: none
Objective: Resolve both review findings.
Work: Add tests first, implement registration and validation, document contract.
Verification: Tests, race, vet, formatting, Linux AMD64 and ARM64 builds.
Evidence: New tests first failed because Command.New and ExecuteFunc did not exist. All CLI tests now pass. Race tests, vet, formatting, diff checks, and Linux AMD64 and ARM64 builds pass. Binary smoke tests confirm JSON version output and exit code 2 for --version info.
## Final verification
All required local checks pass. Independent diff review pending.
## Handoff
Next action: Finish checks, commit and update PR.
Blockers: none
Risks: Option registration must use unique names and must not perform diagnostic work.
