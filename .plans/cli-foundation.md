# Feature: CLI foundation
Status: IN PROGRESS
Issue: https://github.com/daniel-kindl/netknife/issues/4
Branch: feat/4-cli-foundation
PR: not created

## Goal
Provide the CLI foundation and define its output contract.

## Requirements and acceptance criteria
Provide help, version, consistent errors and flags, cancellation, and separate diagnostic results from text and JSON rendering. Document and test the contract.

## In scope
CLI parsing, output rendering, cancellation, tests, and documentation.

## Out of scope
The four diagnostic implementations and release artifacts.

## Dependencies and constraints
Use the Go standard library and support Linux AMD64 and ARM64.

## Decisions and open questions
The owner instructed this session to continue after recording approval on the mapping PR. The PR has no GitHub review entry.

## Commit plan
### C01: Add CLI foundation
Status: IN PROGRESS
Depends on: none
Proposed commit: feat(cli): add command foundation
Actual commit: none
Objective: Define and implement the CLI contract.
Work: Add argument parsing, rendering, cancellation, documentation, and behavior tests.
Verification: Run Go tests, vet, formatting, and Linux builds.
Evidence: not yet run

## Final verification
Run the CI checks and inspect the command output.

## Handoff
Next action: Implement the contract.
Blockers: none
Risks: Later diagnostics must follow the output contract.
