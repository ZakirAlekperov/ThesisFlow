// Package schema manages the database schema compatibility marker.
// Пакет schema ведёт версию структуры локальной базы данных.
package schema

import (
 "context"
 "database/sql"
 "errors"
 "fmt"
)

// CurrentVersion identifies the supported database layout, not the app release.
// RU: CurrentVersion — версия схемы SQLite, а не номер выпуска приложения.
const CurrentVersion = 1

// ErrUnsupportedVersion prevents an older executable from using newer data.
// RU: ErrUnsupportedVersion предотвращает работу старой версии с новой схемой.
var ErrUnsupportedVersion = errors.New("unsupported database schema version")

// Initialize records the initial schema version for both new and pre-versioned
// databases. It never drops or mutates existing domain tables. Future schema
// changes must add explicit numbered migrations rather than changing this ID.
//
// RU: Initialize добавляет маркер совместимости новой или существующей базе.
// Существующие таблицы и данные не изменяются. В следующих версиях схемы
// потребуются отдельные нумерованные миграции, а не замена номера вручную.
func Initialize(ctx context.Context, db *sql.DB) error {
 if err:=ctx.Err();err!=nil{return err}
 tx,err:=db.BeginTx(ctx,nil)
 if err!=nil{return fmt.Errorf("begin schema initialization: %w",err)}
 defer tx.Rollback()
 const create=`CREATE TABLE IF NOT EXISTS app_schema(
  id INTEGER PRIMARY KEY CHECK(id = 1),
  version INTEGER NOT NULL CHECK(version >= 1)
 )`
 if _,err=tx.ExecContext(ctx,create);err!=nil{return fmt.Errorf("create schema marker: %w",err)}
 if _,err=tx.ExecContext(ctx,"INSERT INTO app_schema(id,version) VALUES(1,?) ON CONFLICT(id) DO NOTHING",CurrentVersion);err!=nil{
  return fmt.Errorf("initialize schema version: %w",err)
 }
 var version int
 if err=tx.QueryRowContext(ctx,"SELECT version FROM app_schema WHERE id = 1").Scan(&version);err!=nil{
  return fmt.Errorf("read schema version: %w",err)
 }
 if version!=CurrentVersion{return fmt.Errorf("%w: stored %d, supported %d",ErrUnsupportedVersion,version,CurrentVersion)}
 if err=tx.Commit();err!=nil{return fmt.Errorf("commit schema version: %w",err)}
 return nil
}
