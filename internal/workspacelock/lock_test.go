package workspacelock

import (
 "errors"
 "os"
 "path/filepath"
 "testing"
)

func TestExclusiveLockAndReacquire(t *testing.T){
 dir:=t.TempDir()
 first,err:=Acquire(dir);if err!=nil{t.Fatal(err)}
 defer first.Close()
 if second,err:=Acquire(dir);!errors.Is(err,ErrInUse){
  if second!=nil{second.Close()}
  t.Fatalf("concurrent lock unexpectedly succeeded: %v",err)
 }
 if err:=first.Close();err!=nil{t.Fatal(err)}
 recovered,err:=Acquire(dir)
 if err!=nil{t.Fatalf("released lock not reusable: %v",err)}
 recovered.Close()
 if _,err:=os.Stat(filepath.Join(dir,".thesisflow.lock"));err!=nil{t.Fatal(err)}
}

func TestRefuseSymlinkLock(t *testing.T){
 dir:=t.TempDir()
 target:=filepath.Join(dir,"target")
 if err:=os.WriteFile(target,[]byte("content"),0600);err!=nil{t.Fatal(err)}
 if err:=os.Symlink(target,filepath.Join(dir,".thesisflow.lock"));err!=nil{t.Skipf("symlinks unavailable: %v",err)}
 if locked,err:=Acquire(dir);err==nil{
  locked.Close();t.Fatal("followed symlink lock")
 }
}
