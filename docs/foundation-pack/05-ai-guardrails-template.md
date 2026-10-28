# AI Guardrails Template

## 1. Purpose
Dokumen ini mengatur cara AI bekerja di repo agar tetap disiplin, aman, dan profesional.

## 2. Default Mode
- Default mode: [discussion first / planning first / execute directly for small scoped tasks]
- For large features: discuss architecture before scaffolding.
- For ambiguous requirements: present recommendation, then ask only if ambiguity changes architecture materially.

## 3. Before Touching Code
AI must:
- read relevant files first
- inspect existing conventions before creating new structure
- trace symbol usage before editing
- identify whether the repo already has a pattern to reuse

## 4. Hard Guardrails
- Do not use starter kits without explicit approval.
- Do not add dependencies without approval.
- Do not leave sample/demo/welcome/scaffold residue.
- Do not create new top-level folders without approval.
- Do not do drive-by refactors outside scope.
- Do not invent architecture that is not grounded in the repo or approved blueprint.

## 5. Scaffolding Rules
- Prefer minimal, intentional scaffolding.
- New files must match existing naming and folder conventions.
- If a framework generator creates extra demo files, clean them immediately.
- Avoid generating structure that is not needed in the current phase.

## 6. Editing Rules
- Read before patching.
- Patch the smallest correct surface area.
- Do not rename broad areas unless required.
- If touching one bug path, check sibling paths for the same flaw.

## 7. Planning Rules
Before major implementation, AI should provide:
- scope summary
- module impact
- file impact
- risks
- verification plan

## 8. Verification Rules
AI must verify with the relevant project commands after changes.

Minimum verification checklist:
- formatter: [ISI]
- lint: [ISI]
- test: [ISI]
- build: [ISI]

If a command cannot run, AI must:
- say exactly why
- provide blocker details
- avoid claiming success without evidence

## 9. Communication Rules
- Be concise.
- Lead with the change/result.
- Use assumptions only when marked explicitly.
- Distinguish facts, assumptions, and recommendations.

## 10. Review Rules
Before finalizing, AI should check:
- requirement fit
- architecture fit
- scope discipline
- production quality
- verification evidence

## 11. Refactor Scope Rules
- Small local cleanup is okay if directly adjacent to the task.
- Cross-module refactors require explicit approval.
- Cosmetic refactors are not allowed during business-critical work unless requested.

## 12. Cleanup Rules
After implementation, AI should ensure:
- no placeholder copy remains
- no dead route/page/component from scaffold remains
- no unused broad structure was introduced

## 13. Output Expectations for AI
Every substantial task response should include:
1. what was changed or proposed
2. which files are affected
3. verification performed
4. remaining risk / next step

## 14. Project-Specific Overrides
- [ISI override 1]
- [ISI override 2]
- [ISI override 3]