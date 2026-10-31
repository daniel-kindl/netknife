# Feature: Local network information
Status: IN_PROGRESS
Issue: #5
Branch: feat/5-network-info
PR: not created
Claim-ID: 26a02d96-a553-40d8-918e-25c69250c0ea

## Goal
Implement netknife info with text and JSON output.
## Requirements and acceptance criteria
Local network information, documented behavior, deterministic normal/failure tests, cancellation, ordinary Linux AMD64 and ARM64, no privilege or external tool requirement.
## In scope
All interfaces, index, name, MTU, flags, hardware address and unicast addresses. Stable ordering. Empty arrays. Fail without output on collection errors.
## Out of scope
Routes, DNS configuration, link monitoring, filtering and other diagnostic commands.
## Dependencies and constraints
Standard library only. Preserve the CLI output contract.
## Decisions and open questions
Use net.Interfaces and Interface.Addrs through an injected function adapter. Keep collection in internal/info and command rendering in internal/commands. This is easier to test and change than executing ip and parsing its output. Context checks surround synchronous OS calls; they cannot interrupt those calls. Include down and loopback interfaces. Sort interfaces by index/name and address strings lexically. No partial results.
## Commit plan
### C01: Collect interface data
Status: IN_PROGRESS
Depends on: none
Proposed commit: feat(info): collect local interfaces
Actual commit: none
Objective: Return typed results with clear errors and cancellation.
Work: Tests first, collector, system adapter.
Verification: Deterministic fixture tests for populated, empty, failed and cancelled collection.
Evidence: pending
### C02: Register and document info
Status: PENDING
Depends on: C01
Proposed commit: feat(info): add text and JSON command
Actual commit: none
Objective: Make info available through the CLI.
Work: CLI behavior tests first, command and text renderer, main registration, documentation.
Verification: Unit and CLI tests, live local command, race, vet, formatting and both Linux builds.
Evidence: pending
## Final verification
Pending.
## Handoff
Next action: Write collector tests.
Blockers: none
Risks: Synchronous OS calls do not offer immediate cancellation. Interface configuration can change during collection.
