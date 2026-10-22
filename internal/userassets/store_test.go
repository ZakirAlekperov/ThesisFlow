package userassets

import (
 "bytes"
 "context"
 "errors"
 "os"
 "path/filepath"
 "testing"

 "github.com/ZakirAlekperov/ThesisFlow/internal/groups"
 "github.com/ZakirAlekperov/ThesisFlow/internal/storage/migrate"
 "github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
 "github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

func TestAssetVersionsRemainImmutableAndValidateHash(t *testing.T){
 ctx:=context.Background()
 root:=t.TempDir();db,err:=sqlite.Open(ctx,filepath.Join(root,"thesisflow.sqlite"))
 if err!=nil{t.Fatal(err)};defer db.Close()
 if err:=groups.NewRepository(db).Init(ctx);err!=nil{t.Fatal(err)}
 if err:=students.NewRepository(db).Init(ctx);err!=nil{t.Fatal(err)}
 if err:=migrate.Apply(ctx,db);err!=nil{t.Fatal(err)}
 store:=New(db,filepath.Join(root,"storage"))
 asset,err:=store.Create(ctx,"Глава 1","Образец","Глава 1");if err!=nil{t.Fatal(err)}
 first,err:=store.AddVersion(ctx,asset.ID,"Шаблон.txt",[]byte("Первая версия"))
 if err!=nil{t.Fatal(err)}
 second,err:=store.AddVersion(ctx,asset.ID,"Шаблон.txt",[]byte("Вторая версия"))
 if err!=nil{t.Fatal(err)}
 if second.Number!=2||first.ID==second.ID{t.Fatal("versions not distinct")}
 _,firstBytes,err:=store.Read(ctx,asset.ID,first.ID)
 if err!=nil||string(firstBytes)!="Первая версия"{t.Fatalf("original version lost %q %v",firstBytes,err)}
 _,current,err:=store.Read(ctx,asset.ID,second.ID)
 if err!=nil||string(current)!="Вторая версия"{t.Fatalf("current version %q %v",current,err)}
 if _,_,err:=store.Read(ctx,asset.ID+1,first.ID);!errors.Is(err,ErrInvalid){t.Fatalf("foreign template ID accessed: %v",err)}
 items,err:=store.List(ctx)
 if err!=nil||len(items)!=1||items[0].CurrentVersion!=2{t.Fatalf("asset list %+v %v",items,err)}
 path:=filepath.Join(store.Root,filepath.FromSlash(first.RelativePath))
 if err:=os.WriteFile(path,[]byte("tampered"),0600);err!=nil{t.Fatal(err)}
 if _,_,err:=store.Read(ctx,asset.ID,first.ID);!errors.Is(err,ErrInvalid){t.Fatalf("tampered file not rejected: %v",err)}
}

func TestTemplateRejectsTraversalAndUnsupportedFormats(t *testing.T){
 ctx:=context.Background()
 root:=t.TempDir();db,err:=sqlite.Open(ctx,filepath.Join(root,"thesisflow.sqlite"))
 if err!=nil{t.Fatal(err)};defer db.Close()
 if err:=groups.NewRepository(db).Init(ctx);err!=nil{t.Fatal(err)}
 if err:=students.NewRepository(db).Init(ctx);err!=nil{t.Fatal(err)}
 if err:=migrate.Apply(ctx,db);err!=nil{t.Fatal(err)}
 store:=New(db,filepath.Join(root,"storage"))
 a,err:=store.Create(ctx,"Пример","","");if err!=nil{t.Fatal(err)}
 for _,tc:=range []struct{name string;raw []byte}{
  {"../evil.txt",[]byte("bad")},{"evil.exe",[]byte("bad")},
  {"broken.pdf",[]byte("not PDF")},{"broken.docx",[]byte("not ZIP")},
  {"empty.txt",nil},{"invalid.txt",[]byte{0xff,0xfe}},
 }{
  if _,err:=store.AddVersion(ctx,a.ID,tc.name,tc.raw);!errors.Is(err,ErrInvalid){t.Errorf("accepted %q: %v",tc.name,err)}
 }
 if _,err:=store.AddVersion(ctx,a.ID,"file.txt",bytes.Repeat([]byte("a"),MaxBytes+1));!errors.Is(err,ErrInvalid){t.Fatal(err)}
 versions,err:=store.Versions(ctx,a.ID)
 if err!=nil||len(versions)!=0{t.Fatalf("invalid upload stored %+v %v",versions,err)}
}
