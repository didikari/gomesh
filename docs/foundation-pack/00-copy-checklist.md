# Copy Checklist

Gunakan checklist ini saat menyalin pack ke project baru.

## 1. Copy files minimum
- `docs/foundation-pack/01-project-brief-template.md`
- `docs/foundation-pack/02-prd-template.md`
- `docs/foundation-pack/03-architecture-blueprint-template.md`
- `docs/foundation-pack/04-engineering-guidelines-template.md`
- `docs/foundation-pack/05-ai-guardrails-template.md`
- `docs/foundation-pack/06-implementation-roadmap-template.md`
- `docs/foundation-pack/07-feature-spec-template.md`
- `prompts/01-foundation-generator.md`
- `prompts/02-foundation-finalizer.md`
- `prompts/03-phase-execution.md`

## 2. Rename into active project docs
Disarankan di project baru file finalnya menjadi:
- `docs/project-brief.md`
- `docs/product/prd.md`
- `docs/architecture/overview.md`
- `docs/standards/engineering-guidelines.md`
- `docs/ai/guardrails.md`
- `docs/roadmap/implementation-plan.md`
- `docs/features/<feature-name>.md`

## 3. First fields to replace
Ganti placeholder berikut terlebih dahulu:
- `[ISI NAMA PROJECT]`
- `[ISI DOMAIN / INDUSTRI]`
- `[ISI]` pada stack utama
- hard constraints
- ID policy
- role/permission strategy
- verification commands
- UI quality bar

## 4. Decisions that should be explicit early
- blank foundation atau controlled starter
- auth strategy
- ID strategy
- permission strategy
- architecture style
- testing baseline
- dependency approval policy

## 5. Minimum approval flow
1. Isi project brief
2. Generate/finalkan PRD
3. Finalkan architecture blueprint
4. Finalkan engineering guidelines
5. Finalkan AI guardrails
6. Baru mulai implementasi

## 6. Reminder
Kalau ada bagian yang belum final, jangan dipaksakan. Tandai sebagai `Open Decision` agar AI atau engineer tidak membuat asumsi diam-diam.