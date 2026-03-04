# Sweep planned items after plan completion

After completing [[plan - 2603040954 - migrate observation loop to ai-pipeline-template]], five `{[!]}` items in the ai-pipeline-template spec were still marked as planned even though all were done.
They weren't caught until `/eidos:next` surfaced them.

**Rule:** when marking a plan as completed, sweep the linked spec's `{[!]}` items and mark/remove any that the plan addressed.
This should be part of the plan completion checklist, not a separate cleanup pass.
