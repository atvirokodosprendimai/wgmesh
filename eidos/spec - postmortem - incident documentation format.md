# Spec: postmortem — incident documentation format

TL;DR: A postmortem documents an incident completely enough that a reader with no prior knowledge can understand what happened, why, and what changed.

Mapping: `memory/postmortem - *.md`

---

## Claims

- A postmortem is **blameless**: it focuses on systems and processes, not individuals.
- Required sections (in order): Summary, Impact, Root Causes, Trigger, Detection, Resolution, Action Items, Lessons Learned, Timeline.
- Action items carry explicit **status** (open / done) and an **owner**.
- Lessons Learned has three fixed subsections: **went well**, **went wrong**, **got lucky**.
- Status field follows the lifecycle: `draft` → `in review` → `resolved`.

---

## Structure

### Metadata table

Every postmortem opens with a metadata table:

| Field | Value |
|---|---|
| Title | Short description of the incident |
| Incident # | Sequential integer (001, 002, …) |
| Date | Start date → end date |
| Authors | Who wrote it |
| Status | draft / in review / resolved |
| Duration | Human-readable (e.g. ~10h 52m) |

### Required sections

**Summary**
Two to four sentences.
What happened, what triggered it, and how it ended.

**Impact**
Bullet list.
Who was affected, what was unavailable, and for how long.

**Root Causes**
Numbered list.
Every distinct cause that independently contributed to the incident.
Each cause is bolded with a short label followed by a dash and explanation.

**Trigger**
Single paragraph.
The specific event that started the incident (e.g. a merge, a deploy, a cron job).

**Detection**
Single paragraph.
How the incident was discovered — automated alert, user report, manual observation.

**Resolution**
Chronological table with columns: Time, Commit/PR, Fix description.
Ends with the moment `healthy` state was confirmed.

**Action Items**
Table with columns: #, Item, Status.
Status is `✅ done` or `🔲 open`.
Open items must name an owner when assigned.

**Lessons Learned**
Three fixed subsections — **went well**, **went wrong**, **got lucky** — each a bullet list.
Do not merge or omit any subsection.

**Timeline**
May reference the Resolution table if sufficient.
For complex incidents with parallel threads, use a separate chronological list.

---

## Naming

Files are stored in `memory/` with the prefix pattern:

```
postmortem - <yymmddhhmm> - <claim>.md
```

The timestamp is the resolution time (when the incident was closed), not the start time.
The claim is a short prose description of the incident.

Example: `postmortem - 2602211013 - chimney 503 incident.md`

---

## Notes

- Write the postmortem while the incident is fresh — ideally within 24–48 hours of resolution.
- A postmortem in `draft` status is better than no postmortem; publish and refine.
- Action items that remain open should be linked to corresponding `todo` files in `memory/`.
