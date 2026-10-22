# Code review — ThesisFlow 0.3.0-dev

**Review type:** source inspection + existing Go checks, focused on persisted
student data, HTML UI, version storage, security boundaries, backup and release.
The review is not an external security audit or a formal proof of correctness.
This report distinguishes verified behavior from limitations.

## 1. Package inventory / Пакеты и назначение

| Area | Reviewed files / Scope | Conclusions |
| --- | --- | --- |
| Entry & portability | cmd/thesisflow/main.go, version.go, portable.yml | Loopback HTTP, per-user SQLite path, offline application, Go 1.23; dev version made explicit |
| Persistence | internal/storage/sqlite, storage/migrate, groups, students, requirements | SQLite FK on a single connection; additive migrations; edits preserve IDs; name-change accepted projection requires guard |
| Document lifecycle | fileversions/store.go, review/service.go, materials/repository.go | Immutable version IDs and status events; approved readable file copies are separate; accepted old file remains while new revision is reviewed |
| UI/HTTP | internal/web/handler.go, student_page.go, import_preview.go, document_workflow.go, matrix_page.go | Standard html/template, CSRF tokens, bounded uploads, escaped XLSX names; theme token inconsistencies addressed in #53 |
| Reporting | internal/statusmatrix/matrix.go, readiness and web/dashboard.go | Dynamic summary and per-student material status reads; no denormalized status caching |
| Integrity | internal/integrity/audit.go, fileversions/store.go | Audit checks SHA-256 and path; direct version reads previously lacked these checks; now addressed in this branch |
| Backups | internal/backup and web/backup_restore.go | ZIP backup and isolated restore; restoration does not replace active workspace, backups unencrypted |
| Frontend | theme.css, theme.js, templates | First-party embedded assets, no React/npm; concept C tokens introduced; manual visual acceptance remains |

## 2. Findings ranked / Замечания по приоритету

### High — fixed in this review branch

**Unverified stored version reads.** Earlier `fileversions.Store.Read` used
`filepath.IsLocal` but then performed `os.Open` without rejecting symlinks
and without verifying loaded bytes against recorded size/SHA-256. The version
download route also relied on `Read`. New checks reject unsafe path segments,
symlinks, altered sizes and hashes. Regression tests cover tampering and
a symbolic link aimed outside the workspace.

**Obsolete project description.** Previous README still called the code 0.1.02
and claimed document versions/approval were absent. Replaced with 0.3.0-dev
feature inventory, run/test instructions, safe paths and explicit limitations.

### Medium — fixed in stacked PRs

- **Unreadable active menu** and **dark-mode canvas bands**: CSS used
  conflicting local colors; #53 introduces common palette and viewport styles.
- **Unreadable XLSX preview**: header/cell colors conflicted with the theme;
  #53 introduces semantic headings and scoped contrast.
- **Uneditable group requirements**: #54 adds checked in-place title/deadline
  updates and rejects title changes when an accepted file would retain its old
  human-readable name.
- **Apparently delayed status updates**: #55 introduces a real SQLite +
  separate HTTP-handler acceptance regression for the **first** accepted
  document, plus explicit completion feedback. A browser-specific old-build
  delay is not considered conclusively reproduced.
- **Multipart temp cleanup**: #55 closes the leak in individual uploads.

### Remaining risks / Не устранено

1. **Medium: multi-file imports are not one atomic transaction.**
   The batch upload validates the user input first but filesystem/SQLite
   faults after saving the first item may leave a partly committed batch.
   Responses disclose the number of saved items; add an operation journal
   before offering all-or-nothing batch semantics.
2. **Medium: filesystem path race.** `Read` now rejects symlinks found by
   `Lstat`, but a hostile local process may replace a component between
   checking and opening it (TOCTOU). Strong isolation needs OS-specific
   no-follow/openat-like primitives and permission discipline. No claim of
   complete protection against a same-user malicious process.
3. **Medium: SQLite matrix scalability.** The matrix currently reads each
   student/requirement pair separately, causing O(students × requirements)
   SQL calls. For thousands of students, use one indexed joined/batched query.
4. **Medium: rollback after unexpected process termination.** Database commits
   and filesystem rename operations are not one cross-resource transaction.
   Current normal-error rollback is useful but not crash-proof; add a journal
   and repair procedure before defining an enterprise-grade durability SLA.
5. **Medium: backups are ZIP, not encrypted.** External backup destinations
   and archive sharing must be protected; restoration intentionally creates
   a new directory, not an overwrite.
6. **Low/medium: design duplication.** Old templates still embed per-page CSS
   and separate nav markup. Concept C tokens correct the reported contrasts,
   but moving all shared chrome into reusable Go templates is a later cleanup.
7. **Low: incomplete automatic browser checks.** Existing automated tests cover
   HTTP output and state transitions, not actual visual contrast computed by
   Chromium/Firefox. Run manual screenshots on real platforms; later add
   Playwright only if its maintenance cost is justified.

## 3. Documentation quality / Документирование

- Most externally relevant package behavior has English and Russian doc comments.
  Newly added exported APIs (requirements.Update, ErrAcceptedRename) include
  purpose, limitations, and expected errors.
- Separate RU/EN pages document concept C, requirement editing, status
  freshness, storage integrity, and release acceptance.
- Not every **unexported** field/helper has a line-level comment. That is not
  automatically a defect: comments should describe invariants and non-obvious
  behavior, not narrate obvious assignments. A repository-wide formal
  completeness check remains a separate task; do not claim every line was
  individually reviewed or commented.

## 4. Gates / Проверки перед принятием

```sh
gofmt -l .
go test ./...
go vet ./...
go test -race ./...
```

Run these against the *final integrated merge head*, not only isolated PR
branches. Also test real files through the browser: Excel worksheet preview,
creation/editing of a requirement, first DOCX upload and acceptance, immediate
matrix status, revised document, backup/export/isolated restore, and both
themes on Windows/macOS/Linux. Check hashes and approved-folder structure.
CI success is not a substitute for manual acceptance.

## 5. Development scope / Границы релиза

0.3.0-dev is not a stable 1.0 release. Student accounts, global multi-user
permissions, cloud sync, automatic DOCX/ГОСТ conformance checks, encrypted
backup sharing, and guaranteed atomicity across multiple files are excluded.
