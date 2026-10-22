package main

import (
 "bytes"
 "context"
 "os"
 "path/filepath"
 "testing"

 "github.com/ZakirAlekperov/ThesisFlow/internal/workspaceselect"
)

// TestResetWorkspaceChoiceRecoversFromInvalidNextLaunch tests that an
// administrator can recover after the selected restore was moved or damaged
// without deleting any active or historical files.
// RU: CLI возвращает выбор основного каталога без трогания SQLite.
func TestResetWorkspaceChoiceRecoversFromInvalidNextLaunch(t *testing.T){
 root:=t.TempDir()
 original:=filepath.Join(root,"thesisflow.sqlite")
 if err:=os.WriteFile(original,[]byte("keep unchanged"),0600);err!=nil{t.Fatal(err)}
 marker:=filepath.Join(root,".selected-workspace")
 if err:=os.WriteFile(marker,[]byte("restore-000000000000000000000000\n"),0600);err!=nil{t.Fatal(err)}
 selector:=workspaceselect.New(root)
 if _,_,err:=selector.Resolve();err==nil{t.Fatal("unavailable restored path should prevent startup")}
 var output bytes.Buffer
 if err:=resetWorkspaceChoice(context.Background(),root,&output);err!=nil{t.Fatal(err)}
 chosen,err:=selector.Current()
 if err!=nil||chosen!=workspaceselect.Default{t.Fatalf("reset selection %q %v",chosen,err)}
 data,err:=os.ReadFile(original)
 if err!=nil||string(data)!="keep unchanged"{t.Fatalf("reset changed original: %q %v",data,err)}
}
