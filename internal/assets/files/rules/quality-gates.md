# Rules — SDD Quality Gates

## Never declare "Done/Ready" without:

- A spec sufficient for the phase (or open questions explicitly recorded)
- Clear acceptance criteria
- Documented edge cases
- Relevant tests passing
- Unit coverage >= 80%

## "Ready" per phase (summary)

- **Discover**: scope, stories, acceptance criteria, open questions
- **Design**: contracts, NFRs, tradeoffs, risks
- **Tasks**: small tasks, each with acceptance + tests
- **Implement**: real evidence (commands, outputs)
- **Review**: objective findings list with severity + actions
- **Ship**: checklist, rollout/rollback, release notes
