package schema

import (
 "context"
 "database/sql"
 "errors"
 "path/filepath"
 "testing"

 "github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
)

// testDB creates a disposable SQLite workspace for migration tests.
// RU: testDB создаёт отдельную временную SQLite для испытаний миграций.
func testDB(t *testing.T) *sql.DB {
 t.Helper()
 db,err:=sqlite.Open(context.Background(),filepath.Join(t.TempDir(),"thesisflow.sqlite"))
 if err!=nil { t.Fatal(err) }
 t.Cleanup(func(){ _=db.Close() })
 return db
}

// TestInitializeCreatesVersionAndCanRepeat verifies idempotent startup.
// RU: Проверяет создание таблицы версий и повторную инициализацию.
func TestInitializeCreatesVersionAndCanRepeat(t *testing.T) {
 db:=testDB(t)
 for range 2 {
  if err:=Initialize(context.Background(),db);err!=nil {t.Fatal(err)}
 }
 var version int
 if err:=db.QueryRow("SELECT version FROM app_schema WHERE id = 1").Scan(&version);err!=nil {t.Fatal(err)}
 if version!=CurrentVersion {t.Errorf("version=%d, want=%d",version,CurrentVersion)}
}

// TestInitializeAcceptsExistingLegacyDB verifies migration from pre-versioned
// workspaces without deleting existing user rows.
// RU: Проверяет сохранение старых данных при добавлении таблицы версий.
func TestInitializeAcceptsExistingLegacyDB(t *testing.T) {
 db:=testDB(t)
 if _,err:=db.Exec("CREATE TABLE legacy(value TEXT)");err!=nil {t.Fatal(err)}
 if _,err:=db.Exec("INSERT INTO legacy(value) VALUES ('preserve me')");err!=nil {t.Fatal(err)}
 if err:=Initialize(context.Background(),db);err!=nil {t.Fatal(err)}
 var value string
 if err:=db.QueryRow("SELECT value FROM legacy").Scan(&value);err!=nil {t.Fatal(err)}
 if value!="preserve me" {t.Errorf("legacy data lost: %q",value)}
}

// TestInitializeRejectsFutureVersion prevents old binaries from modifying a
// database created by newer ThesisFlow versions.
// RU: Проверяет запрет запуска устаревшей программы на новой базе.
func TestInitializeRejectsFutureVersion(t *testing.T) {
 db:=testDB(t)
 if err:=Initialize(context.Background(),db);err!=nil {t.Fatal(err)}
 if _,err:=db.Exec("UPDATE app_schema SET version = 999 WHERE id = 1");err!=nil {t.Fatal(err)}
 if err:=Initialize(context.Background(),db);!errors.Is(err,ErrUnsupportedVersion) {t.Errorf("error=%v, want ErrUnsupportedVersion",err)}
}

// TestInitializeRespectsCanceledContext does not change state after cancellation.
// RU: Проверяет, что отменённая операция не создаёт таблицы.
func TestInitializeRespectsCanceledContext(t *testing.T) {
 db:=testDB(t)
 ctx,cancel:=context.WithCancel(context.Background());cancel()
 if err:=Initialize(ctx,db);!errors.Is(err,context.Canceled) {t.Errorf("error=%v",err)}
 var found int
 if err:=db.QueryRow("SELECT count(*) FROM sqlite_master WHERE name = 'app_schema'").Scan(&found);err!=nil {t.Fatal(err)}
 if found!=0 {t.Fatal("schema table created after canceled context")}
}
