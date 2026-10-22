# ThesisFlow 0.3.0-dev — review and acceptance / Приёмка

This is a development candidate, not a stable release or an automatically
merged branch. / Это ветка разработки, а не объявление стабильного выпуска.

## Stacked pull requests / Порядок рассмотрения

- #48: consolidated 0.2.0 base (already merged, but historical branches remain).
- #49–#51: thesis workspace, batch uploads, and status filters.
- #53: approved concept C visual system, dark/light contrast and XLSX preview.
- #54: group requirement title/due-date editing.
- #55: immediate status matrix regression and clearer approval feedback.
- The current audit branch: README/version refresh and safer version reads.

### Core acceptance / Обязательные проверки

1. Create a fictional group; import a real XLSX after selecting a worksheet.
2. Create all ten standard thesis document requirements.
3. Edit one title and due date before first acceptance.
4. Upload a DOCX, accept it, and check the **first** accepted state in the
   matrix immediately. No second document should be required.
5. Attempt to rename the requirement with an accepted file; expect refusal.
   Deadline-only changes should still work.
6. Replace an accepted document with a revised version; verify history and
   the accepted copy preserve the old approved bytes until new approval.
7. Test corrupt/symlink version reads and run the integrity audit.
8. Check light/dark pages at the top/bottom, Excel preview contrast and
   active button colors.
9. Export backup, restore to a different directory, verify recovered data.
10. Run gofmt, tests, vet, race checks and portable builds on integrated head.

## Release notes / Ограничения

Not shipped: web-based student portal, staff roles, cloud sync, encryption of
backups, full DOCX formatting standards check, or atomic all-or-nothing file
batch saves. The GitHub PR owner performs merge/release decisions.
