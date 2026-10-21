package migrate

import (
    "context"
    "database/sql"
    "errors"
    "path/filepath"
    "testing"

    "github.com/ZakirAlekperov/ThesisFlow/internal/groups"
    "github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
    "github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// newLegacyDB creates a pre-migration workspace using the existing repository.
// RU: newLegacyDB создаёт прежнюю схему через действующие репозитории.
func newLegacyDB(t *testing.T) *sql.DB {
    t.Helper()
    ctx:=context.Background()
    db,err:=sqlite.Open(ctx,filepath.Join(t.TempDir(),"legacy.sqlite"))
    if err!=nil {t.Fatal(err)}
    t.Cleanup(func(){_ = db.Close()})
    if err:=groups.NewRepository(db).Init(ctx);err!=nil{t.Fatal(err)}
    if err:=students.NewRepository(db).Init(ctx);err!=nil{t.Fatal(err)}
    return db
}

// TestApplyPreservesLegacyData checks migration compatibility with real user
// records. Scenario: insert a group and student, then apply migration.
// Expected: both IDs and names survive, new columns/tables exist.
//
// RU: Проверяет сохранность старых групп и студентов после обновления схемы.
// Сценарий: запись старых данных, запуск миграции.
// Ожидается: прежние ID, ФИО, группы и новые поля без потери данных.
func TestApplyPreservesLegacyData(t *testing.T) {
    ctx:=context.Background()
    db:=newLegacyDB(t)
    group,err:=groups.NewRepository(db).Create(ctx,"ИСП-401")
    if err!=nil{t.Fatal(err)}
    student,err:=students.NewRepository(db).Create(ctx,group.ID,"TF20-001","Иванов Иван")
    if err!=nil{t.Fatal(err)}
    if err:=Apply(ctx,db);err!=nil{t.Fatal(err)}
    var full string
    if err:=db.QueryRowContext(ctx,"SELECT full_name FROM students WHERE id = ?",student.ID).Scan(&full);err!=nil{t.Fatal(err)}
    if full != student.FullName {t.Fatalf("student lost or modified: %q",full)}
    var archived sql.NullString
    if err:=db.QueryRowContext(ctx,"SELECT archived_at FROM student_groups WHERE id = ?",group.ID).Scan(&archived);err!=nil{t.Fatal(err)}
    if archived.Valid {t.Fatal("existing group unexpectedly archived")}
    var count int
    if err:=db.QueryRowContext(ctx,"SELECT COUNT(*) FROM thesis_records").Scan(&count);err!=nil{t.Fatal(err)}
    if count!=0 {t.Fatalf("unexpected theses: %d",count)}
    var version int
    if err:=db.QueryRowContext(ctx,"PRAGMA user_version").Scan(&version);err!=nil{t.Fatal(err)}
    if version!=CurrentVersion {t.Fatalf("version %d, want %d",version,CurrentVersion)}
}

// TestApplyCanRunAgain verifies migration is idempotent for existing users.
// Scenario: initialize schema twice. Expected: no error or changed version.
//
// RU: Проверяет безопасный повторный запуск без повторного ALTER TABLE.
func TestApplyCanRunAgain(t *testing.T) {
    db:=newLegacyDB(t)
    for i:=0;i<2;i++ { if err:=Apply(context.Background(),db);err!=nil{t.Fatalf("run %d: %v",i,err)} }
}

// TestApplyRejectsNewerDatabase prevents silently opening future formats.
// Scenario: PRAGMA user_version exceeds supported version.
// Expected: ErrFutureVersion and unchanged schema version.
//
// RU: Проверяет запрет открытия базы более новой версии приложения.
func TestApplyRejectsNewerDatabase(t *testing.T) {
    db:=newLegacyDB(t)
    if _,err:=db.Exec("PRAGMA user_version = 999");err!=nil{t.Fatal(err)}
    if err:=Apply(context.Background(),db); !errors.Is(err,ErrFutureVersion){t.Fatalf("error %v, want ErrFutureVersion",err)}
    var version int
    if err:=db.QueryRow("PRAGMA user_version").Scan(&version);err!=nil{t.Fatal(err)}
    if version != 999 {t.Fatal("newer version overwritten")}
}

// TestApplyDoesNotRunWithCanceledContext guards shutdown/startup races.
// RU: Проверяет, что отмена контекста не запускает обновление схемы.
func TestApplyDoesNotRunWithCanceledContext(t *testing.T) {
    db:=newLegacyDB(t)
    ctx,cancel:=context.WithCancel(context.Background());cancel()
    if err:=Apply(ctx,db);!errors.Is(err,context.Canceled){t.Fatalf("got %v",err)}
}
