# Architecture Blueprint Template

## 1. Blueprint Summary
- System name: [ISI]
- Architecture style: [monolith / modular monolith / service-oriented / hybrid]
- Primary stack: [ISI]
- Key architectural goals:
  - [ISI]
  - [ISI]

## 2. System Context
- Primary actors:
  - [ISI]
- External systems/integrations:
  - [ISI]
- Deployment context:
  - [ISI]

## 3. High-Level Architecture
### Major layers
- Presentation layer: [ISI]
- Application layer: [ISI]
- Domain/business layer: [ISI]
- Data layer: [ISI]

### Boundary rules
- [contoh: controller hanya orchestration]
- [contoh: validation di request object]
- [contoh: business rule tidak menumpuk di page/component]

## 4. Core Modules
### Module: [ISI]
- Purpose: [ISI]
- Main responsibilities:
  - [ISI]
- Depends on:
  - [ISI]

### Module: [ISI]
- Purpose: [ISI]
- Main responsibilities:
  - [ISI]
- Depends on:
  - [ISI]

## 5. Domain Model
### Main entities
- [ENTITY]: [deskripsi]
- [ENTITY]: [deskripsi]
- [ENTITY]: [deskripsi]

### Key relationships
- [ENTITY A] -> [ENTITY B]: [relasi]
- [ENTITY A] -> [ENTITY C]: [relasi]

### Identifier policy
- Business entities use: [UUID / ULID / integer / hybrid]
- Public routes use: [ISI]

## 6. Role & Permission Model
- [ROLE]: [scope akses]
- [ROLE]: [scope akses]
- Permission strategy: [simple role column / policy-driven / permission package]

## 7. Main Workflows
### Workflow: [ISI]
1. [ISI]
2. [ISI]
3. [ISI]
4. [ISI]

### Workflow: [ISI]
1. [ISI]
2. [ISI]
3. [ISI]
4. [ISI]

## 8. Backend Conventions
- Controllers: [ISI pola]
- Requests/validation: [ISI pola]
- Actions/services: [ISI pola]
- Policies/authorization: [ISI pola]
- Resources/DTOs: [ISI pola]
- Events/jobs: [ISI pola]

## 9. Frontend Conventions
- Page organization: [ISI]
- Layout strategy: [ISI]
- Component hierarchy: [page / section / feature / primitive]
- Form strategy: [ISI]
- Data table strategy: [ISI]
- State/data fetching strategy: [ISI]

## 10. Routing Strategy
- Web routes: [ISI]
- API routes: [ISI]
- Naming conventions: [ISI]
- URL conventions: [ISI]

## 11. Data & Persistence Strategy
- Main database: [ISI]
- Migration policy: [ISI]
- Soft delete policy: [ISI]
- Audit log policy: [ISI]
- Seeding strategy: [ISI]

## 12. Recommended Folder Structure
```text
[ISI struktur folder yang disepakati]
```

## 13. Cross-Cutting Concerns
- Logging/observability: [ISI]
- Error handling: [ISI]
- Security: [ISI]
- Performance: [ISI]
- Background jobs: [ISI]

## 14. Risks
- [RISIKO]: [kenapa berisiko]
  - Mitigation: [ISI]
- [RISIKO]: [kenapa berisiko]
  - Mitigation: [ISI]

## 15. Open Decisions
- [ISI]
- [ISI]