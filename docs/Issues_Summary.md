# Issues_Summary

Open workable items (current_location = Issues), regenerated from the SQLite single-source-of-truth (§11.4.12).

## Counts by Type × Status

| Type | Status | Count |
|---|---|---|
| Feature | Queued | 2 |
| Task | Queued | 3 |
| **TOTAL** | | **5** |

## Items

| ATM ID | Type | Status | Severity | Description |
|---|---|---|---|---|
| HXC-118 | Feature | Queued | High | A dedicated Retrieval-Augmented-Generation component is maintained as its own reusable module, but the main application does not import or use it anywhere. A capability the product is expected to offer (answering using retrieved documents) is therefore effectively unavailable to end users despite the code existing. The work integrates the existing RAG module into the application, wires it into the request flow, and exposes its capability flag. Users gain working document-grounded answers instead of an orphaned, unused component. |
| HXC-119 | Feature | Queued | High | Governance rule CONST-040 lists the Agent Client Protocol among required capabilities, but there is no implementation of it anywhere in the codebase. Any user or integration expecting ACP connectivity currently cannot use it. The work is to design and implement real ACP support, or, if it proves structurally infeasible, to document that with cited evidence. The platform will then either genuinely support ACP or hold an honest, evidenced position instead of an unmet claim. |
| HXC-122 | Task | Queued | Medium | Two categories of tests, memory-usage and end-to-end automation, skip most of their cases by default because they require a live server or special environment flags not set in normal runs. In practice these areas are largely unverified even though the tests appear to exist. The work provides a documented, repeatable way to run them against real infrastructure so they actually execute and prove the behavior. Memory and automation behavior then becomes genuinely tested rather than merely scaffolded. |
| HXC-136 | Task | Queued | Medium | Several mandated automated test categories — load/denial-of-service, scaling, stress and chaos, and user-interface/experience — were not exercised in the latest real-infrastructure run, so their current health is unconfirmed. The work is to run each of these test types against real infrastructure and capture proof of the results. This completes the promised full test-type coverage and confirms the product holds up under load and adverse conditions. |
| HXC-138 | Task | Queued | Low | The end-to-end challenge runner can now launch all its scenarios (a missing option was just fixed), but the scenarios still need to be executed against a live server with a real model to confirm the complete user journeys work. The work is to stand up a server and run the challenges, capturing the results. This provides real proof that the headline user workflows function end to end. |
