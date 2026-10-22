package review

import (
 "context"
 "errors"
 "os"
 "path/filepath"
 "strings"
 "testing"

)

var simulatedCrash=errors.New("simulated process interruption")

// TestApprovalRecoveryAtAllCrashBoundaries simulates losing the process
// between durable intent, backup rename, file publication and SQL commit.
// It then creates a new Service and replays the outstanding durable record.
// RU: Аварии до и после SQL commit не уничтожают предыдущую принятую версию.
func TestApprovalRecoveryAtAllCrashBoundaries(t *testing.T) {
 for _,stage:=range []string{"after_intent","after_old_renamed","after_publish","after_commit"}{
  t.Run(stage,func(t *testing.T){
   ctx:=context.Background()
   service,studentID,requirementID:=testReview(t)
   first,err:=service.Versions.Save(ctx,studentID,requirementID,"first.docx",[]byte("accepted old document"))
   if err!=nil{t.Fatal(err)}
   previous,err:=service.Accept(ctx,studentID,requirementID,first.ID)
   if err!=nil{t.Fatal(err)}
   second,err:=service.Versions.Save(ctx,studentID,requirementID,"second.docx",[]byte("new document version"))
   if err!=nil{t.Fatal(err)}
   service.interruptApproval=func(at string)error {
    if at==stage{return simulatedCrash}
    return nil
   }
   if _,err:=service.Accept(ctx,studentID,requirementID,second.ID);!errors.Is(err,simulatedCrash){
    t.Fatalf("injection %s did not interrupt: %v",stage,err)
   }
   pending,err:=service.PendingCount(ctx)
   if err!=nil||pending!=1{t.Fatalf("not durable journal: count=%d %v",pending,err)}
   // New approvals cannot proceed before the previous intent is resolved.
   if _,err:=service.Accept(ctx,studentID,requirementID,second.ID);!errors.Is(err,ErrPendingApproval){
    t.Fatalf("accepted a second operation while unresolved: %v",err)
   }
   restarted:=NewService(service.DB,service.Versions)
   if err:=restarted.Init(ctx);err!=nil{t.Fatal(err)}
   recovered,err:=restarted.RecoverPending(ctx)
   if err!=nil||recovered!=1{t.Fatalf("restart recovery %d: %v",recovered,err)}
   recovered,err=restarted.RecoverPending(ctx)
   if err!=nil||recovered!=0{t.Fatalf("non-idempotent replay: %d %v",recovered,err)}
   copyPath:=filepath.Join(service.Versions.Root,filepath.FromSlash(previous.RelativePath))
   got,err:=os.ReadFile(copyPath)
   if err!=nil{t.Fatal(err)}
   expected:="accepted old document"
   wantVersion:=first.ID
   if stage=="after_commit"{
    expected="new document version"
    wantVersion=second.ID
   }
   if string(got)!=expected{t.Errorf("after %s recovered bytes %q wanted %q",stage,got,expected)}
   var currentVersion int64
   if err:=service.DB.QueryRowContext(ctx,`SELECT version_id FROM accepted_files
    WHERE student_id=? AND requirement_id=?`,studentID,requirementID).Scan(&currentVersion);err!=nil{t.Fatal(err)}
   if currentVersion!=wantVersion{t.Errorf("recovered SQLite version %d expected %d",currentVersion,wantVersion)}
   var approvals int
   if err:=service.DB.QueryRowContext(ctx,`SELECT COUNT(*) FROM material_events
    WHERE student_id=? AND requirement_id=? AND status='accepted'`,
    studentID,requirementID).Scan(&approvals);err!=nil{t.Fatal(err)}
   wantEvents:=1
   if stage=="after_commit"{wantEvents=2}
   if approvals!=wantEvents{t.Errorf("acceptance events %d expected %d",approvals,wantEvents)}
   matches,err:=filepath.Glob(copyPath+".backup-*")
   if err!=nil{t.Fatal(err)}
   if len(matches)!=0{t.Errorf("stale protected backup after recovery: %v",matches)}
  })
 }
}

// TestApprovalRecoveryHandlesNewPathWithoutDeletingOld checks a document
// extension change and interrupted publication to a different filename.
// RU: При изменении расширения старая утверждённая копия сохраняется.
func TestApprovalRecoveryHandlesNewPathWithoutDeletingOld(t *testing.T){
 for _,stage:=range []string{"after_publish","after_commit"}{
  t.Run(stage,func(t *testing.T){
   ctx:=context.Background()
   svc,studentID,requirementID:=testReview(t)
   a,err:=svc.Versions.Save(ctx,studentID,requirementID,"part.docx",[]byte("old-docx"))
   if err!=nil{t.Fatal(err)}
   old,err:=svc.Accept(ctx,studentID,requirementID,a.ID);if err!=nil{t.Fatal(err)}
   b,err:=svc.Versions.Save(ctx,studentID,requirementID,"part.pdf",[]byte("%PDF-1.4 new"))
   if err!=nil{t.Fatal(err)}
   svc.interruptApproval=func(point string)error {
    if point==stage{return simulatedCrash}
    return nil
   }
   _,err=svc.Accept(ctx,studentID,requirementID,b.ID)
   if !errors.Is(err,simulatedCrash){t.Fatalf("expected injected crash: %v",err)}
   next:=NewService(svc.DB,svc.Versions)
   if _,err:=next.RecoverPending(ctx);err!=nil{t.Fatal(err)}
   oldPath:=filepath.Join(svc.Versions.Root,filepath.FromSlash(old.RelativePath))
   data,err:=os.ReadFile(oldPath)
   if stage=="after_publish"{
    if err!=nil||string(data)!="old-docx"{t.Errorf("prior projection lost: %q %v",data,err)}
   }else if !errors.Is(err,os.ErrNotExist){
    t.Errorf("stale old projection remains after new commit: %v %q",err,data)
   }
   newPath:=strings.TrimSuffix(oldPath,".docx")+".pdf"
   data,err=os.ReadFile(newPath)
   if stage=="after_publish" {
    if !errors.Is(err,os.ErrNotExist){t.Errorf("orphan unaccepted PDF remained: %v",err)}
   }else if err!=nil||string(data)!="%PDF-1.4 new"{
    t.Errorf("committed PDF not published: %q %v",data,err)
   }
  })
 }
}

// TestRecoveryRefusesUnknownModifiedFile protects manually changed student
// documents and keeps unresolved intents for an explicit repair decision.
// RU: Не стираем неизвестные байты, даже при наличии SQLite-намерения.
func TestRecoveryRefusesUnknownModifiedFile(t *testing.T){
 ctx:=context.Background()
 svc,studentID,ruleID:=testReview(t)
 old,err:=svc.Versions.Save(ctx,studentID,ruleID,"old.txt",[]byte("accepted"))
 if err!=nil{t.Fatal(err)}
 accepted,err:=svc.Accept(ctx,studentID,ruleID,old.ID)
 if err!=nil{t.Fatal(err)}
 replace,err:=svc.Versions.Save(ctx,studentID,ruleID,"new.txt",[]byte("replacement"))
 if err!=nil{t.Fatal(err)}
 svc.interruptApproval=func(stage string)error {
  if stage=="after_publish"{return simulatedCrash}
  return nil
 }
 if _,err:=svc.Accept(ctx,studentID,ruleID,replace.ID);!errors.Is(err,simulatedCrash){t.Fatal(err)}
 dest:=filepath.Join(svc.Versions.Root,filepath.FromSlash(accepted.RelativePath))
 if err:=os.WriteFile(dest,[]byte("manual third-party edit"),0600);err!=nil{t.Fatal(err)}
 next:=NewService(svc.DB,svc.Versions)
 if _,err:=next.RecoverPending(ctx);!errors.Is(err,ErrRecoveryConflict){
  t.Fatalf("unknown edited content overwritten: %v",err)
 }
 got,err:=os.ReadFile(dest)
 if err!=nil||string(got)!="manual third-party edit"{t.Fatalf("edited content destroyed: %q %v",got,err)}
 pending,err:=next.PendingCount(ctx)
 if err!=nil||pending!=1{t.Fatalf("conflict was hidden: %d %v",pending,err)}
}

func TestAcceptRejectsTamperedPriorCopyWithoutCreatingIntent(t *testing.T){
 ctx:=context.Background()
 svc,studentID,ruleID:=testReview(t)
 v,err:=svc.Versions.Save(ctx,studentID,ruleID,"old.docx",[]byte("accepted"))
 if err!=nil{t.Fatal(err)}
 accepted,err:=svc.Accept(ctx,studentID,ruleID,v.ID)
 if err!=nil{t.Fatal(err)}
 path:=filepath.Join(svc.Versions.Root,filepath.FromSlash(accepted.RelativePath))
 if err:=os.WriteFile(path,[]byte("teacher manual edits"),0600);err!=nil{t.Fatal(err)}
 newVersion,err:=svc.Versions.Save(ctx,studentID,ruleID,"new.docx",[]byte("new revision"))
 if err!=nil{t.Fatal(err)}
 if _,err:=svc.Accept(ctx,studentID,ruleID,newVersion.ID);!errors.Is(err,ErrRecoveryConflict){
  t.Fatalf("silently overwrote edited accepted file: %v",err)
 }
 pending,err:=svc.PendingCount(ctx)
 if err!=nil||pending!=0{t.Fatalf("rejected approval recorded intent: %d %v",pending,err)}
}
