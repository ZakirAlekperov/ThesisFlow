package requirements

import (
 "context"
 "database/sql"
 "errors"
 "fmt"
 "strings"
 "time"
)

// ErrAcceptedRename prevents title edits that would desynchronize the visible
// accepted filename from its document requirement.
// RU: Переименование документа с принятым файлом требует безопасного переноса.
var ErrAcceptedRename = errors.New("accepted document name cannot be changed without relocating its file")

// Update changes a group-wide document requirement in place, retaining its
// permanent ID and existing material versions. Group ID and student scope
// cannot be changed through this operation. Valid title changes are rejected
// when an accepted file exists, protecting the human-readable projection.
//
// RU: Update изменяет общее требование группы, сохраняя ID, версии и статусы.
// Если уже есть принятый файл, его название не меняется, а срок — может.
func (r *Repository) Update(ctx context.Context, groupID, requirementID int64, title, dueDate string) (Requirement, error) {
 if err := ctx.Err(); err != nil { return Requirement{}, err }
 if groupID <= 0 || requirementID <= 0 { return Requirement{}, ErrInvalidScope }
 title = strings.TrimSpace(title)
 dueDate = strings.TrimSpace(dueDate)
 if title == "" { return Requirement{}, ErrInvalidTitle }
 if dueDate != "" {
  date, err := time.Parse("2006-01-02", dueDate)
  if err != nil || date.Format("2006-01-02") != dueDate { return Requirement{}, ErrInvalidDate }
 }
 tx, err := r.db.BeginTx(ctx, nil)
 if err != nil { return Requirement{}, fmt.Errorf("begin requirement update: %w", err) }
 defer tx.Rollback()
 var previous string
 err = tx.QueryRowContext(ctx,
  "SELECT title FROM requirements WHERE id=? AND group_id=? AND student_id IS NULL",
  requirementID, groupID).Scan(&previous)
 if errors.Is(err, sql.ErrNoRows) { return Requirement{}, ErrInvalidScope }
 if err != nil { return Requirement{}, fmt.Errorf("load requirement: %w", err) }
 if title != previous {
  // The earliest schema may not contain accepted_files yet.
  // RU: Старые базы могут не иметь таблицы принятых документов.
  var acceptedTable int
  err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='accepted_files')").Scan(&acceptedTable)
  if err != nil { return Requirement{}, fmt.Errorf("locate approval index: %w", err) }
  if acceptedTable == 1 {
   var approved int
   err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM accepted_files WHERE requirement_id=?)", requirementID).Scan(&approved)
   if err != nil { return Requirement{}, fmt.Errorf("check accepted files: %w", err) }
   if approved == 1 { return Requirement{}, ErrAcceptedRename }
  }
 }
 normalized := strings.ToLower(title)
 var collision int
 err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM requirements WHERE group_id=? AND student_id IS NULL AND title_key=? AND id<>?)",
  groupID, normalized, requirementID).Scan(&collision)
 if err != nil { return Requirement{}, fmt.Errorf("check duplicate requirement: %w", err) }
 if collision == 1 { return Requirement{}, ErrDuplicateTitle }
 result, err := tx.ExecContext(ctx,
  "UPDATE requirements SET title=?, title_key=?, due_date=? WHERE id=? AND group_id=? AND student_id IS NULL",
  title, normalized, dueDate, requirementID, groupID)
 if err != nil { return Requirement{}, fmt.Errorf("update requirement: %w", err) }
 affected, err := result.RowsAffected()
 if err != nil { return Requirement{}, fmt.Errorf("count updated requirements: %w", err) }
 if affected != 1 { return Requirement{}, ErrInvalidScope }
 if err := tx.Commit(); err != nil { return Requirement{}, fmt.Errorf("commit requirement update: %w", err) }
 return Requirement{ID: requirementID, GroupID: groupID, Title: title, DueDate: dueDate}, nil
}
