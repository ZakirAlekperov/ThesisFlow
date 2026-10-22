package userassets

import (
 "context"
 "errors"
 "os"
 "path/filepath"
 "testing"

 "github.com/ZakirAlekperov/ThesisFlow/internal/backup"
 "github.com/ZakirAlekperov/ThesisFlow/internal/groups"
 "github.com/ZakirAlekperov/ThesisFlow/internal/requirements"
 "github.com/ZakirAlekperov/ThesisFlow/internal/storage/migrate"
 "github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
 "github.com/ZakirAlekperov/ThesisFlow/internal/students"
 "github.com/ZakirAlekperov/ThesisFlow/internal/workspace"
)
func TestTemplateBindingBackupRestoreAndIntegrity(t *testing.T){
 ctx:=context.Background()
 base:=t.TempDir()
 source:=filepath.Join(base,"source")
 db,err:=sqlite.Open(ctx,filepath.Join(source,"thesisflow.sqlite"));if err!=nil{t.Fatal(err)}
 defer db.Close()
 gr:=groups.NewRepository(db);sr:=students.NewRepository(db)
 for _,init:=range []func(context.Context)error{gr.Init,sr.Init}{if err:=init(ctx);err!=nil{t.Fatal(err)}}
 if err:=migrate.Apply(ctx,db);err!=nil{t.Fatal(err)}
 rr:=requirements.NewRepository(db);if err:=rr.Init(ctx);err!=nil{t.Fatal(err)}
 g,err:=gr.Create(ctx,"ИСП-401");if err!=nil{t.Fatal(err)}
 req,err:=rr.Create(ctx,g.ID,0,"Глава 1","");if err!=nil{t.Fatal(err)}
 store:=New(db,filepath.Join(source,"storage"))
 asset,err:=store.Create(ctx,"Главы","Образец","Глава 1");if err!=nil{t.Fatal(err)}
 version,err:=store.AddVersion(ctx,asset.ID,"template.txt",[]byte("Document template content"))
 if err!=nil{t.Fatal(err)}
 if err:=store.Bind(ctx,g.ID,req.ID,asset.ID);err!=nil{t.Fatal(err)}
 rows,err:=store.ListBindings(ctx,g.ID)
 if err!=nil||len(rows)!=1||rows[0].AssetID!=asset.ID{t.Fatalf("binding %+v %v",rows,err)}
 verified,err:=workspace.Check(ctx,db,store.Root)
 if err!=nil||!verified.Healthy()||verified.TemplatesChecked!=1||verified.TemplatesIntact!=1||verified.ExtraFiles!=0{
  t.Fatalf("template health %+v %v",verified,err)
 }
 archive:=filepath.Join(base,"copy.zip")
 if err:=backup.Export(ctx,db,store.Root,archive);err!=nil{t.Fatal(err)}
 restored:=filepath.Join(base,"restored")
 if err:=backup.Restore(archive,restored);err!=nil{t.Fatalf("restore %+v",err)}
 restoredDB,err:=sqlite.Open(ctx,filepath.Join(restored,"thesisflow.sqlite"))
 if err!=nil{t.Fatal(err)}
 defer restoredDB.Close()
 copyStore:=New(restoredDB,filepath.Join(restored,"storage"))
 _,content,err:=copyStore.Read(ctx,asset.ID,version.ID)
 if err!=nil||string(content)!="Document template content"{t.Fatalf("restored bytes %q %v",content,err)}
 if err:=os.WriteFile(filepath.Join(store.Root,filepath.FromSlash(version.RelativePath)),[]byte("corrupted"),0600);err!=nil{t.Fatal(err)}
 bad,err:=workspace.Check(ctx,db,store.Root)
 if err!=nil||bad.Healthy(){t.Fatalf("template corruption missed %+v %v",bad,err)}
 if err:=backup.Export(ctx,db,store.Root,filepath.Join(base,"invalid.zip"));!errors.Is(err,backup.ErrUnsafeArchive){
  t.Fatalf("corrupt template backup accepted: %v",err)
 }
 if err:=store.Bind(ctx,g.ID,req.ID,0);err!=nil{t.Fatal(err)}
 rows,err=store.ListBindings(ctx,g.ID)
 if err!=nil||rows[0].AssetID!=0{t.Fatalf("asset unlink failed %+v %v",rows,err)}
}
