package workspace

import (
 "context"
 "database/sql"
 "errors"
 "fmt"
 "io/fs"
 "os"
 "path/filepath"

 "github.com/ZakirAlekperov/ThesisFlow/internal/storagepath"
)

// inventoryUnknown counts unregistered regular files without reading their
// content, following symlinks or removing anything. Warnings are advisory:
// existing v1 archives can legitimately contain historical extras.
// RU: Посторонние файлы только подсчитываются, их нельзя удалять автоматически.
func inventoryUnknown(ctx context.Context, db *sql.DB, root string) (int, error) {
 if err:=ctx.Err();err!=nil{return 0,err}
 known:=make(map[string]struct{})
 for _,table:=range []string{"file_versions","accepted_files"} {
  present,err:=tableExists(ctx,db,table)
  if err!=nil{return 0,err}
  if !present{continue}
  rows,err:=db.QueryContext(ctx,"SELECT relative_path FROM "+table)
  if err!=nil{return 0,fmt.Errorf("list registered paths: %w",err)}
  for rows.Next(){
   var raw string
   if err:=rows.Scan(&raw);err!=nil{rows.Close();return 0,err}
   normalized,err:=storagepath.Normalize(raw)
   if err==nil{known[normalized]=struct{}{}}
  }
  if err:=rows.Err();err!=nil{rows.Close();return 0,err}
  if err:=rows.Close();err!=nil{return 0,err}
 }
 info,err:=os.Lstat(root)
 if errors.Is(err,os.ErrNotExist){return 0,nil}
 if err!=nil{return 0,err}
 if !info.IsDir() || info.Mode()&os.ModeSymlink!=0{return 0,errors.New("unsafe storage directory")}
 unknown:=0
 err=filepath.WalkDir(root,func(item string,d fs.DirEntry,walkErr error)error{
  if walkErr!=nil{return walkErr}
  if err:=ctx.Err();err!=nil{return err}
  if item==root{return nil}
  // Do not traverse symlink targets, including a symlink to a directory.
  if d.IsDir(){return nil}
  rel,err:=filepath.Rel(root,item)
  if err!=nil{return err}
  native:=filepath.ToSlash(rel)
  norm,err:=storagepath.Normalize(native)
  if err!=nil{return fmt.Errorf("unsafe stored directory entry: %w",err)}
  if _,ok:=known[norm];!ok{unknown++}
  return nil
 })
 if err!=nil{return 0,fmt.Errorf("inventory storage files: %w",err)}
 return unknown,nil
}

// UnknownWarnings describes a non-destructive, advisory inventory result.
// RU: Предупреждение не приравнивается к повреждению текущих документов.
func (r Report) UnknownWarnings() []string {
 if r.ExtraFiles==0{return nil}
 return []string{fmt.Sprintf("Unregistered storage entries: %d",r.ExtraFiles)}
}

// inspectOrphans enriches the health report without failing an otherwise
// healthy legacy workspace simply because historical files are untracked.
// RU: Наличие старых лишних файлов не блокирует проверку ZIP.
func inspectOrphans(ctx context.Context,db *sql.DB,root string,report *Report)error{
 count,err:=inventoryUnknown(ctx,db,root)
 if err!=nil{return err}
 report.ExtraFiles=count
 return nil
}
