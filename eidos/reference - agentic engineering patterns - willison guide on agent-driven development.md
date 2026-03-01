---
tldr: Simon Willison's collected patterns for working effectively with coding agents — testing, economics, comprehension, prompts
---

# Reference: Agentic Engineering Patterns (Simon Willison)

## What It Is

A guide by Simon Willison collecting practical patterns for getting better results from coding agents (Claude Code, Codex, etc.).
Not theory — field-tested habits from someone shipping daily with agents.

## Key Patterns

### 1. Writing code is cheap now

Agent-generated code is near-free to produce.
But **good code** is still expensive — correctness, security, maintainability, tests, docs.
The shift: reconsider things previously rejected as "not worth the dev time."
Fire off a prompt in an async agent session — worst case you check 10 minutes later and discard.

### 2. Hoard things you know how to do

Accumulate a personal library of solved problems and working examples.
Agents excel at recombining existing solutions — give them material to work with.
TIL notes, repos, tools, documented patterns compound over time.
Once you've solved a problem and documented it, agents reuse it across future projects.

### 3. Red/green TDD

"Use red/green TDD" — four words that encode a full discipline.
Write tests first, confirm they **fail** (red), then implement until they pass (green).
Critical for agents: prevents non-functional code, stops unnecessary code, builds regression safety net.
Skipping the "confirm tests fail" step risks tests that pass trivially.

### 4. First run the tests

Start every agent session with "first run the tests" (or the project-specific command).
Three purposes:
- Agent discovers the test suite exists and learns how to run it
- Test count signals project size and complexity
- Puts agent in a testing mindset — it'll expand the suite naturally

### 5. Linear walkthroughs

Ask the agent to "read the source and plan a linear walkthrough explaining how it all works."
Useful for code you forgot, vibe-coded, or inherited.
Showboat pattern: agent uses `sed`/`grep`/`cat` to quote real code (prevents hallucination).

### 6. Interactive explanations

Combat cognitive debt from agent-written code by requesting animated/interactive demos.
Agent builds HTML visualisations showing algorithms step-by-step.
Turns black-box code into intuitive understanding.

### 7. Prompts appendix

- Keep custom instructions minimal and specific (font size, indentation, framework bans)
- Hard boundary: LLMs don't write opinion content or first-person voice
- Proofreading prompt: spelling, grammar, repetition, weak arguments, empty links

## How We Use It

- **Testing discipline**: our goose-review recipe and company-loop both benefit from "first run the tests" — see [[spec - first-customer - roadmap to first paying customer]]
- **Hoarding patterns**: eidos itself is a hoarding system — specs, references, decisions compound across sessions
- **Code-is-cheap mindset**: company control loop was iterated 6+ times in one session — each fix-merge-trigger cycle took minutes
- **Linear walkthroughs**: `/eidos:pull` does exactly this — reverse-engineer understanding from code into specs

## Sources

- [Agentic Engineering Patterns (guide index)](https://simonwillison.net/guides/agentic-engineering-patterns/) — the full guide
- [Writing code is cheap now](https://simonwillison.net/guides/agentic-engineering-patterns/code-is-cheap/)
- [Hoard things you know how to do](https://simonwillison.net/guides/agentic-engineering-patterns/hoard-things-you-know-how-to-do/)
- [Red/green TDD](https://simonwillison.net/guides/agentic-engineering-patterns/red-green-tdd/)
- [First run the tests](https://simonwillison.net/guides/agentic-engineering-patterns/first-run-the-tests/)
- [Linear walkthroughs](https://simonwillison.net/guides/agentic-engineering-patterns/linear-walkthroughs/)
- [Interactive explanations](https://simonwillison.net/guides/agentic-engineering-patterns/interactive-explanations/)
- [Prompts I use](https://simonwillison.net/guides/agentic-engineering-patterns/prompts/)
