package main

import (
 "bytes"
 "context"
 "errors"
 "os"
 "path/filepath"
 "strings"
 "testing"

 "github.com/ZakirAlekperov/ThesisFlow/internal/backup"
 "github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
)

// TestDoctorNeverCreatesMissingDatabase preserves existing data and rejects
// a missing workspace without starting the browser.
// RU: Doctor не создаёт пустую БД вместо отсутствующих пользовательских данных.
func TestDoctorNeverCreatesMissingDatabase(t *testing.T){
 root:=t.TempDir()
 var output bytes.Buffer
 err:=runDoctorCommand(context.Background(),root,&output)
 if err==nil{t.Fatal("doctor accepted missing database")}
 if _,err:=os.Stat(filepath.Join(root,"thesisflow.sqlite"));!os.IsNotExist(err){
  t.Errorf("doctor created empty database: %v",err)
 }
}

func TestDoctorAndVerifyBackupFromRealSQLite(t *testing.T){
 ctx:=context.Background()
 root:=t.TempDir()
 db,err:=sqlite.Open(ctx,filepath.Join(root,"thesisflow.sqlite"))
 if err!=nil{t.Fatal(err)}
 if _,err:=db.ExecContext(ctx,"CREATE TABLE sample (text TEXT)");err!=nil{t.Fatal(err)}
 if err:=db.Close();err!=nil{t.Fatal(err)}
 var output bytes.Buffer
 if err:=runDoctorCommand(ctx,root,&output);err!=nil{t.Fatal(err)}
 if !strings.Contains(output.String(),"критических нарушений не обнаружено"){t.Fatal(output.String())}
 db,err=sqlite.Open(ctx,filepath.Join(root,"thesisflow.sqlite"))
 if err!=nil{t.Fatal(err)}
 archive:=filepath.Join(root,"copy.zip")
 if err:=backup.Export(ctx,db,filepath.Join(root,"storage"),archive);err!=nil{t.Fatal(err)}
 if err:=db.Close();err!=nil{t.Fatal(err)}
 output.Reset()
 if err:=runBackupCommand(ctx,root,[]string{"backup","verify",archive},&output);err!=nil{t.Fatal(err)}
 if !strings.Contains(output.String(),"прошла проверку"){t.Fatal(output.String())}
 // Alter the already verified ZIP: it must fail, without altering workspace.
 if err:=os.WriteFile(archive,[]byte("broken archive"),0600);err!=nil{t.Fatal(err)}
 if err:=runBackupCommand(ctx,root,[]string{"backup","verify",archive},&output);err==nil{
  t.Fatal("corrupt archive verified successfully")
 }
 if errors.Is(runDoctorCommand(ctx,root,&output),ErrUnhealthyWorkspace){
  t.Fatal("archive verification changed active workspace")
 }
}
