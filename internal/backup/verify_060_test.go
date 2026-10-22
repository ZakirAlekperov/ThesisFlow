package backup

import (
 "archive/zip"
 "context"
 "errors"
 "os"
 "path/filepath"
 "testing"

 "github.com/ZakirAlekperov/ThesisFlow/internal/fileversions"
 "github.com/ZakirAlekperov/ThesisFlow/internal/groups"
 "github.com/ZakirAlekperov/ThesisFlow/internal/materials"
 "github.com/ZakirAlekperov/ThesisFlow/internal/requirements"
 "github.com/ZakirAlekperov/ThesisFlow/internal/review"
 "github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
 "github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

func reviewBackupFixture(t *testing.T)(string,string,*review.Service,int64,int64,int64) {
 t.Helper()
 ctx:=context.Background()
 root:=t.TempDir()
 db,err:=sqlite.Open(ctx,filepath.Join(root,"source","thesisflow.sqlite"))
 if err!=nil{t.Fatal(err)}
 t.Cleanup(func(){_ = db.Close()})
 gs:=groups.NewRepository(db);people:=students.NewRepository(db)
 req:=requirements.NewRepository(db);statuses:=materials.NewRepository(db)
 versions:=fileversions.NewStore(db,filepath.Join(root,"source","storage"))
 checks:=review.NewService(db,versions)
 for _,init:=range []func(context.Context)error{gs.Init,people.Init,req.Init,statuses.Init,versions.Init,checks.Init}{
  if err:=init(ctx);err!=nil{t.Fatal(err)}
 }
 g,err:=gs.Create(ctx,"ИС-401");if err!=nil{t.Fatal(err)}
 student,err:=people.Create(ctx,g.ID,"301001","Иванов Иван");if err!=nil{t.Fatal(err)}
 required,err:=req.Create(ctx,g.ID,0,"Глава 1","");if err!=nil{t.Fatal(err)}
 version,err:=versions.Save(ctx,student.ID,required.ID,"Глава 1.docx",[]byte("a real saved version"))
 if err!=nil{t.Fatal(err)}
 if _,err:=checks.Accept(ctx,student.ID,required.ID,version.ID);err!=nil{t.Fatal(err)}
 return root,filepath.Join(root,"source","storage"),checks,student.ID,required.ID,version.ID
}

// TestRestoreAndVerifyRejectSemanticallyCorruptZIP rehashes a broken accepted
// projection inside a perfectly self-consistent ZIP manifest.
// RU: Корректный ZIP с повреждённой связью файлов всё равно отклоняется.
func TestRestoreAndVerifyRejectSemanticallyCorruptZIP(t *testing.T){
 root,storage,svc,studentID,requirementID,_:=reviewBackupFixture(t)
 ctx:=context.Background()
 good:=filepath.Join(root,"good.zip")
 if err:=Export(ctx,svc.DB,storage,good);err!=nil{t.Fatal(err)}
 if err:=Verify(good);err!=nil{t.Fatalf("healthy backup rejected: %v",err)}
 destination:=filepath.Join(root,"restored")
 if err:=Restore(good,destination);err!=nil{t.Fatalf("restore healthy ZIP: %v",err)}

 var relative string
 if err:=svc.DB.QueryRowContext(ctx,
  "SELECT relative_path FROM accepted_files WHERE student_id=? AND requirement_id=?",
  studentID,requirementID).Scan(&relative);err!=nil{t.Fatal(err)}
 if err:=os.WriteFile(filepath.Join(storage,relative),[]byte("wrong bytes"),0600);err!=nil{t.Fatal(err)}
 broken:=filepath.Join(root,"damaged.zip")
 if err:=Export(ctx,svc.DB,storage,broken);err!=nil{t.Fatal(err)}
 if err:=Verify(broken);!errors.Is(err,ErrUnsafeArchive){
  t.Errorf("verified a semantically broken backup: %v",err)
 }
 unsafeDest:=filepath.Join(root,"wrongly-restored")
 if err:=Restore(broken,unsafeDest);!errors.Is(err,ErrUnsafeArchive){
  t.Errorf("restored a broken backup: %v",err)
 }
 if _,err:=os.Stat(unsafeDest);!os.IsNotExist(err){
  t.Errorf("corrupt ZIP published a destination: %v",err)
 }
}

func TestVerifyRejectsFutureSQLiteSchema(t *testing.T){
 ctx:=context.Background()
 root,storage,svc,_,_,_:=reviewBackupFixture(t)
 if _,err:=svc.DB.ExecContext(ctx,"PRAGMA user_version = 500");err!=nil{t.Fatal(err)}
 archive:=filepath.Join(root,"future.zip")
 if err:=Export(ctx,svc.DB,storage,archive);err!=nil{t.Fatal(err)}
 if err:=Verify(archive);!errors.Is(err,ErrUnsafeArchive){
  t.Fatalf("future schema accepted: %v",err)
 }
}

func TestRestoreRejectsCaseCollisionsAndWindowsReservedNames(t *testing.T){
 for _,names:=range [][]string{
  {"storage/Chapter.docx","storage/chapter.docx"},
  {"storage/CON.txt"},
  {"storage/LPT1.pdf"},
  {"storage/Chapter. "},
 }{
  root:=t.TempDir()
  archive:=filepath.Join(root,"case.zip")
  file,err:=os.Create(archive)
  if err!=nil{t.Fatal(err)}
  z:=zip.NewWriter(file)
  for _,name:=range names{
   w,err:=z.Create(name);if err!=nil{t.Fatal(err)}
   if _,err:=w.Write([]byte("payload"));err!=nil{t.Fatal(err)}
  }
  if err:=z.Close();err!=nil{t.Fatal(err)}
  if err:=file.Close();err!=nil{t.Fatal(err)}
  err=Restore(archive,filepath.Join(root,"unsafe-target"))
  if !errors.Is(err,ErrUnsafeArchive){t.Errorf("accepted ambiguous or reserved archive %v: %v",names,err)}
 }
}
