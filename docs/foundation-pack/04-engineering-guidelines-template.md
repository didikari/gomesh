# Engineering Guidelines Template

## 1. Purpose
Dokumen ini menjadi standar engineering utama untuk repo ini.

## 2. Repo Principles
- [ISI prinsip 1]
- [ISI prinsip 2]
- [ISI prinsip 3]

## 3. Architecture Discipline
- Jangan menaruh business logic berat di controller/page.
- Ikuti boundary layer yang disepakati pada architecture blueprint.
- Hindari abstraksi dini yang belum terbukti dibutuhkan.
- Jangan membuat helper/generic utility baru tanpa alasan jelas.

## 4. Folder Conventions
- Backend structure: [ISI]
- Frontend structure: [ISI]
- Tests structure: [ISI]
- Shared/support code: [ISI]

## 5. Naming Conventions
- Entities: [ISI]
- Controllers: [ISI]
- Requests: [ISI]
- Actions/services: [ISI]
- Pages/components: [ISI]
- Routes: [ISI]
- Database tables/columns: [ISI]

## 6. Backend Rules
- Controllers should: [ISI]
- Validation should live in: [ISI]
- Authorization should use: [ISI]
- Query complexity should move to: [ISI]
- Resources/DTO formatting should use: [ISI]
- Error handling pattern: [ISI]

## 7. Frontend Rules
- Page-level components should: [ISI]
- Shared components should: [ISI]
- Form handling pattern: [ISI]
- Table/listing pattern: [ISI]
- Empty/loading/error state expectations: [ISI]
- UI quality bar: [ISI]

## 8. Data & Database Rules
- ID policy: [UUID / ULID / integer / hybrid]
- Foreign key naming: [ISI]
- Soft delete usage: [ISI]
- Audit/history requirement: [ISI]
- Migration safety rules:
  - [ISI]
  - [ISI]

## 9. Dependency Policy
- Allowed by default:
  - framework-native features
  - existing approved packages
- Requires approval:
  - new package install
  - framework starter kits
  - infrastructure-affecting change
  - major architecture refactor

## 10. Testing Policy
- Required test types: [ISI]
- Critical flows that must be covered:
  - [ISI]
  - [ISI]
- Minimum verification commands:
  - [ISI]
  - [ISI]
  - [ISI]

## 11. Code Review Expectations
Review changes for:
- correctness
- scope discipline
- consistency with conventions
- maintainability
- testability
- UX production quality

## 12. Definition of Done
Sebuah task dianggap selesai jika:
- kebutuhan utama terpenuhi
- tidak keluar scope
- struktur code konsisten dengan blueprint
- lint/build/test yang relevan lulus
- tidak meninggalkan scaffold/demo residue
- risiko atau follow-up dicatat jelas

## 13. Explicit Anti-Patterns
- [ISI anti-pattern 1]
- [ISI anti-pattern 2]
- [ISI anti-pattern 3]

## 14. Open Decisions
- [ISI]
- [ISI]