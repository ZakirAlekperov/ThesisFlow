package workspaceselect

import (
 "context"
 "errors"
 "os"
 "path/filepath"
 "strings"
 "testing"

 "github.com/ZakirAlekperov/ThesisFlow/internal/backup"
 "github.com/ZakirAlekperov/ThesisFlow/internal/groups"
 "github.com/ZakirAlekperov/ThesisFlow/internal/storage/migrate"
 "github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
 "github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

func recoveredFixture(t *testing.T)(*Service,string){
 t.Helper()
 ctx:=context.Background()
 home:=t.TempDir()
 db,err:=sqlite.Open(ctx,filepath.Join(home,"thesisflow.sqlite"))
 if err!=nil{t.Fatal(err)}
 defer db.Close()
 if err:=groups.NewRepository(db).Init(ctx);err!=nil{t.Fatal(err)}
 if err:=students.NewRepository(db).Init(ctx);err!=nil{t.Fatal(err)}
 if err:=migrate.Apply(ctx,db);err!=nil{t.Fatal(err)}
 name:="restore-0123456789abcdef01234567"
 archive:=filepath.Join(t.TempDir(),"backup.zip")
 if err:=backup.Export(ctx,db,filepath.Join(home,"storage"),archive);err!=nil{t.Fatal(err)}
 dir:=filepath.Join(home,"restores",name)
 if err:=backup.Restore(archive,dir);err!=nil{t.Fatal(err)}
 return New(home),name
}

// TestSelectOnlyVerifiedRestoresForNextRun verifies that the chosen DB is a
// valid restored snapshot while the default SQLite bytes remain untouched.
// RU: Переключение выбирает файл, не перемещая действующую SQLite.
func TestSelectOnlyVerifiedRestoresForNextRun(t *testing.T){
 s,name:=recoveredFixture(t)
 before,err:=os.ReadFile(filepath.Join(s.Root,"thesisflow.sqlite"))
 if err!=nil{t.Fatal(err)}
 options,err:=s.List(context.Background())
 if err!=nil||len(options)!=1||options[0].ID!=name{t.Fatalf("list %+v error %v",options,err)}
 choice,err:=s.Current()
 if err!=nil||choice!=Default{t.Fatalf("default selection %q %v",choice,err)}
 if err:=s.Choose(context.Background(),name);err!=nil{t.Fatalf("select restore: %v",err)}
 choice,err=s.Current()
 if err!=nil||choice!=name{t.Fatalf("selected id %q, err %v",choice,err)}
 path,selected,err:=s.Resolve()
 if err!=nil||!selected||path!=filepath.Join(s.Root,"restores",name,"thesisflow.sqlite"){
  t.Fatalf("unexpected next-launch choice %q %v %v",path,selected,err)
 }
 after,err:=os.ReadFile(filepath.Join(s.Root,"thesisflow.sqlite"))
 if err!=nil||string(before)!=string(after){t.Fatalf("selection modified original database: %v",err)}
 if err:=s.Choose(context.Background(),Default);err!=nil{t.Fatal(err)}
 path,selected,err=s.Resolve()
 if err!=nil||selected||path!=filepath.Join(s.Root,"thesisflow.sqlite"){
  t.Fatalf("return to default %q %v %v",path,selected,err)
 }
}

// TestSelectRejectsTraversalSymlinksAndInvalidState validates the browser
// selection is NOT an arbitrary filesystem path or implicit fallback.
// RU: Подмена указателя не должна открывать незнакомую БД.
func TestSelectRejectsTraversalSymlinksAndInvalidState(t *testing.T){
 s,valid:=recoveredFixture(t)
 for _,name:=range []string{"../private","/etc","restore-../../bad","restore-AAAA","restore-ffffffffffffffffffffffff","restore-0123456789abcdef01234567/..",""}{
  if err:=s.Choose(context.Background(),name);!errors.Is(err,ErrInvalidChoice){
   t.Errorf("unsafe selection %q returned %v",name,err)
  }
 }
 if err:=s.Choose(context.Background(),valid);err!=nil{t.Fatal(err)}
 if err:=os.WriteFile(s.path(),[]byte("../private\n"),0600);err!=nil{t.Fatal(err)}
 if _,_,err:=s.Resolve();!errors.Is(err,ErrInvalidChoice){t.Fatalf("invalid marker silently fell back: %v",err)}
 if err:=os.Remove(s.path());err!=nil{t.Fatal(err)}
 target:=filepath.Join(s.Root,"badmarker")
 if err:=os.WriteFile(target,[]byte("default\n"),0600);err!=nil{t.Fatal(err)}
 if err:=os.Symlink(target,s.path());err==nil{
  if _,err:=s.Current();!errors.Is(err,ErrInvalidChoice){t.Fatalf("symlinked marker read: %v",err)}
 } else{t.Logf("symlinks unavailable: %v",err)}
}

// TestSelectionRefusesCorruptionBeforePersisting checks verified restoration
// cannot be selected after its database is corrupted, and old choice survives.
// RU: Плохая копия не может стать активной на следующем запуске.
func TestSelectionRefusesCorruptionBeforePersisting(t *testing.T){
 s,name:=recoveredFixture(t)
 if err:=s.Choose(context.Background(),Default);err!=nil{t.Fatal(err)}
 dbPath:=filepath.Join(s.Root,"restores",name,"thesisflow.sqlite")
 db,err:=sqlite.Open(context.Background(),dbPath)
 if err!=nil{t.Fatal(err)}
 if _,err:=db.Exec("PRAGMA user_version=999");err!=nil{t.Fatal(err)}
 if err:=db.Close();err!=nil{t.Fatal(err)}
 if err:=s.Choose(context.Background(),name);!errors.Is(err,ErrInvalidChoice){t.Fatalf("future schema accepted: %v",err)}
 choice,err:=s.Current()
 if err!=nil||choice!=Default{t.Errorf("bad selection overwrote old choice: %s %v",choice,err)}
 db,err=sqlite.Open(context.Background(),dbPath)
 if err!=nil{t.Fatal(err)}
 if _,err:=db.Exec("PRAGMA user_version=1");err!=nil{t.Fatal(err)}
 db.Close()
 if err:=os.WriteFile(dbPath,[]byte(strings.Repeat("X",8192)),0600);err!=nil{t.Fatal(err)}
 if err:=s.Choose(context.Background(),name);err==nil{t.Fatal("corrupted SQLite selected")}
}
