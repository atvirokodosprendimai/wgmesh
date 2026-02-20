# This is the seed for your specifications

The main idea:
- You can create code from intent (prompts)
- You can extract intent from code (=>docs)
- Eidos helps you keep intent (spec docs) and manifestation (code) in sync
- Eidos lets you work more on the intent layer (edit specs)

Run `/eidos:help` for more info.

## New Projects

Replace the content of this file with all your project relevant notes.
Then tell claude to run `/eidos:spec` on it.
Take a look at the generated files.
And leave inline comments using `{{double curly braces}}`.
Just write casually on what should be changed, or what general thoughts you have.
Then run `/eidos:refine` to have the ai process and group them.
It may ask you more questions, which you can answer.
You may end up creating a `/eidos:plan` to start making larger spec edits, or start to implement things.
You can also use `/eidos:reference` to write docs on knowledge that is relevant to your project, but not in its control.

## Existing Projects

If you have any notes you want to incorporate, see the point above.
Otherwise try out `/eidos:pull` to extract a base specification from your existing code.
Read it through, and then either edit directly or leave `{{comments}}` to `/eidos:refine` with LLM support.
