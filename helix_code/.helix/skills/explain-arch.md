---
description: Explain a named subsystem's architecture in HelixCode.
triggers:
  - "explain (?P<topic>\\w+) architecture"
variables:
  audience: engineer
---
# Architecture: {{ARG.topic}}

Produce a concise architecture explanation of the **{{ARG.topic}}** subsystem for a {{ARG.audience}} audience.

Cover:
- The role {{ARG.topic}} plays in HelixCode and its public entry points.
- Key types and the data flow through {{ARG.topic}}.
- How {{ARG.topic}} is wired into the rest of the system.

Keep it focused on {{ARG.topic}}; do not drift into unrelated subsystems.
