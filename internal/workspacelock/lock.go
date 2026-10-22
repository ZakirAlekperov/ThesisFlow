// Package workspacelock prevents multiple ThesisFlow processes from
// modifying the same local workspace simultaneously.
// RU: Межпроцессная блокировка защищает рабочую папку.
package workspacelock

import (
 "errors"
 "fmt"
 "os"
 "path/filepath"
)

var ErrInUse=errors.New("workspace is already open by another process")

// Lock is held until Close; the sentinel file is intentionally retained so
// stale filenames never masquerade as held OS locks.
// RU: Файл блокировки не удаляется; ОС снимает блокировку при завершении.
type Lock struct{file *os.File}

// Acquire obtains an exclusive nonblocking OS file lock for this workspace.
// It never deletes stale lock files or follows a symlink at the lock filename.
// RU: Файловый дескриптор владеет блокировкой до Close или конца процесса.
func Acquire(root string)(*Lock,error){
 if root==""{return nil,errors.New("empty workspace directory")}
 if err:=os.MkdirAll(root,0700);err!=nil{return nil,err}
 rootInfo,err:=os.Lstat(root)
 if err!=nil{return nil,err}
 if !rootInfo.IsDir()||rootInfo.Mode()&os.ModeSymlink!=0{return nil,errors.New("workspace root is not a safe directory")}
 name:=filepath.Join(root,".thesisflow.lock")
 info,err:=os.Lstat(name)
 if err!=nil&&!errors.Is(err,os.ErrNotExist){return nil,err}
 if err==nil&&(!info.Mode().IsRegular()||info.Mode()&os.ModeSymlink!=0){return nil,errors.New("unsafe lock file")}
 file,err:=os.OpenFile(name,os.O_CREATE|os.O_RDWR,0600)
 if err!=nil{return nil,err}
 if err:=lockFile(file);err!=nil{
  _=file.Close()
  return nil,fmt.Errorf("%w: %v",ErrInUse,err)
 }
 return &Lock{file:file},nil
}

// Close releases the advisory OS lock; safe at most once.
// RU: Закрытие блокировки при завершении процесса.
func(l *Lock)Close()error{
 if l==nil||l.file==nil{return nil}
 f:=l.file;l.file=nil
 err:=unlockFile(f)
 closeErr:=f.Close()
 if err!=nil{return err}
 return closeErr
}
