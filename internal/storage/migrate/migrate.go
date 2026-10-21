// Package migrate upgrades existing ThesisFlow SQLite databases in place.
// Пакет migrate обновляет существующую схему SQLite без потери данных.
package migrate

import (
 "context"
 "database/sql"
 "errors"
 "fmt"
)

// CurrentVersion is the highest SQLite user_version understood by this build.
// RU: CurrentVersion — последняя версия схемы, поддерживаемая приложением.
const CurrentVersion = 1

// ErrFutureVersion prevents opening a database made by a newer application.
// RU: ErrFutureVersion защищает базы, созданные более новой версией.
var ErrFutureVersion = errors.New("database schema is newer than this application")

// Apply runs all schema upgrades in one transaction. It must be called after
// the existing group and student tables have been initialized; it preserves
// existing records and commits the version change with the new columns.
//
// RU: Apply выполняет миграции атомарно после создания базовых таблиц групп
// и студентов. Сведения пользователей сохраняются, номер схемы обновляется
// в той же транзакции, что и структура таблиц.
func Apply(ctx context.Context, db *sql.DB) error {
 if err:=ctx.Err();err!=nil{return err}
 tx,err:=db.BeginTx(ctx,nil)
 if err!=nil{return fmt.Errorf("start database migration: %w",err)}
 defer tx.Rollback()
 var version int
 if err:=tx.QueryRowContext(ctx,"PRAGMA user_version").Scan(&version);err!=nil {
  return fmt.Errorf("read SQLite schema version: %w",err)
 }
 if version > CurrentVersion {return fmt.Errorf("%w: got %d, supports %d",ErrFutureVersion,version,CurrentVersion)}
 if version == 0 {
  statements:=[]string{
   "ALTER TABLE student_groups ADD COLUMN archived_at TEXT",
   "ALTER TABLE students ADD COLUMN archived_at TEXT",
   `CREATE TABLE IF NOT EXISTS thesis_records (
       student_id INTEGER PRIMARY KEY REFERENCES students(id) ON DELETE RESTRICT,
       topic TEXT NOT NULL DEFAULT '',
       order_reference TEXT NOT NULL DEFAULT '',
       updated_at TEXT NOT NULL DEFAULT (datetime('now'))
   )`,
   "PRAGMA user_version = 1",
  }
  for _,statement:=range statements {
   if _,err:=tx.ExecContext(ctx,statement);err!=nil {
    return fmt.Errorf("apply SQLite migration to version 1: %w",err)
   }
  }
 }
 if err:=tx.Commit();err!=nil{return fmt.Errorf("commit SQLite migration: %w",err)}
 return nil
}
