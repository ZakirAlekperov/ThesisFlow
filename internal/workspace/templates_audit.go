package workspace

import (
 "context"
 "database/sql"
 "errors"
 "fmt"
 "os"
 "strings"

 "github.com/ZakirAlekperov/ThesisFlow/internal/storagepath"
)

// inspectTemplateVersions verifies every immutable teacher template recorded
// in SQLite. Missing or damaged bytes are blocking integrity issues for
// backup and recovery, just like student document versions.
// RU: Пользовательские шаблоны участвуют в проверке ZIP и восстановлении.
func inspectTemplateVersions(ctx context.Context, db *sql.DB, root string, report *Report)error{
 present,err:=tableExists(ctx,db,"template_versions")
 if err!=nil{return err}
 if !present{return nil} // old v1 workspace
 rows,err:=db.QueryContext(ctx,"SELECT id,relative_path,size,sha256 FROM template_versions ORDER BY id")
 if err!=nil{return err}
 defer rows.Close()
 for rows.Next(){
  if err:=ctx.Err();err!=nil{return err}
  var id,size int64
  var relative,digest string
  if err:=rows.Scan(&id,&relative,&size,&digest);err!=nil{return err}
  report.TemplatesChecked++
  normalized,err:=storagepath.Normalize(relative)
  if err!=nil||!strings.HasPrefix(normalized,".internal/templates/")||size<0||size>8<<20||len(digest)!=64{
   report.add("template_metadata",0,0,id,"registered template path or digest is unsafe")
   continue
  }
  file,err:=canonicalPath(root,normalized)
  if errors.Is(err,os.ErrNotExist){
   report.add("template_missing",0,0,id,"registered teacher template missing")
   continue
  }
  if err!=nil{
   report.add("template_unsafe_path",0,0,id,"unsafe teacher template path")
   continue
  }
  actualSize,hash,err:=hashFile(file,8<<20)
  if err!=nil||actualSize!=size||!strings.EqualFold(hash,digest){
   report.add("template_corrupt",0,0,id,"teacher template checksum mismatch")
   continue
  }
  report.TemplatesIntact++
 }
 if err:=rows.Err();err!=nil{return fmt.Errorf("read teacher template registry: %w",err)}
 return nil
}
