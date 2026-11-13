# Lastapp foundational engineering principles

**DRY (Don't Repeat Yourself) is the main, foundational principle of this system.**

- Every business rule, policy, constant and shared contract has one authoritative source. Derive consumers from that source; do not copy definitions across endpoints, forms, languages or documentation that serves as an executable contract.
- The backend owns security policies. The UI consumes the public policy contract; never hardcode its own password limits or fallback policy. Share form controls and field definitions across all account-creation and password-change flows.
- A policy change must cover every consumer, including first-login onboarding, ordinary account settings, administrator/account creation and generated passwords. Regression tests must verify behavior against the authoritative contract and a different supplied policy, not just repeat today's constants.
- Keep it simple. Reuse existing components and APIs; do not add parallel implementations or an abstraction without a concrete need.
- Inspect and preserve existing changes before editing. Do not change the pinned MariaDB driver for unrelated application work.
