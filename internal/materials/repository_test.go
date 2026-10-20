package materials

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/requirements"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// setupMaterialStatus prepares student and document requirements in SQLite.
// RU: setupMaterialStatus готовит студента и требования в тестовой SQLite.
func setupMaterialStatus(t *testing.T) (*Repository,*sql.DB,int64,int64) {
	t.Helper()
	ctx:=context.Background()
	db,err:=sqlite.Open(ctx,filepath.Join(t.TempDir(),"thesisflow.sqlite"))
	if err!=nil { t.Fatal(err) }
	t.Cleanup(func(){_ = db.Close()})
	g:=groups.NewRepository(db)
	s:=students.NewRepository(db)
	req:=requirements.NewRepository(db)
	for _,init:=range []func(context.Context)error{g.Init,s.Init,req.Init} {
		if err:=init(ctx);err!=nil {t.Fatal(err)}
	}
	group,err:=g.Create(ctx,"ИСП-401");if err!=nil {t.Fatal(err)}
	student,err:=s.Create(ctx,group.ID,"ЗК-001","Иванов Иван");if err!=nil {t.Fatal(err)}
	doc,err:=req.Create(ctx,group.ID,0,"Глава 1","");if err!=nil {t.Fatal(err)}
	status:=NewRepository(db)
	if err:=status.Init(ctx);err!=nil {t.Fatal(err)}
	return status,db,student.ID,doc.ID
}

// TestStatusDefaultsToMissing verifies a new student's document is not yet
// submitted without creating fake submission or event rows.
// RU: Проверяет состояние «Не сдан» до реальной регистрации сдачи.
func TestStatusDefaultsToMissing(t *testing.T) {
	repo,_,studentID,requirementID:=setupMaterialStatus(t)
	entry,err:=repo.Get(context.Background(),studentID,requirementID)
	if err!=nil || entry.Status!=Missing {t.Fatalf("Get = %+v, %v",entry,err)}
	events,err:=repo.History(context.Background(),studentID,requirementID)
	if err!=nil || len(events)!=0 {t.Errorf("history = %v, err=%v",events,err)}
}

// TestStatusTransitionsRecordImmutableHistory protects the revision history.
// Scenario: mark submitted, return for revisions, accept.
// Expected: current status accepted with three ordered persisted events.
//
// RU: Проверяет историю сдачи, доработки и приёмки.
// Сценарий: «На проверке» → «На доработке» → «Принят».
// Ожидается: три события в истории и текущий статус «Принят».
func TestStatusTransitionsRecordImmutableHistory(t *testing.T) {
	repo,_,studentID,requirementID:=setupMaterialStatus(t)
	now:=time.Date(2026,10,20,12,0,0,0,time.UTC)
	repo.Clock=func()time.Time{return now}
	for _,status:=range []Status{UnderReview,NeedsRevision,Accepted} {
		entry,err:=repo.Set(context.Background(),studentID,requirementID,status)
		if err!=nil || entry.Status!=status {t.Fatalf("Set(%s) = %+v, %v",status,entry,err)}
		now=now.Add(time.Hour)
	}
	entry,err:=repo.Get(context.Background(),studentID,requirementID)
	if err!=nil || entry.Status!=Accepted {t.Fatalf("Get = %+v, %v",entry,err)}
	events,err:=repo.History(context.Background(),studentID,requirementID)
	if err!=nil || len(events)!=3 {t.Fatalf("History = %+v, %v",events,err)}
	if events[0].Status!=UnderReview || events[1].Status!=NeedsRevision || events[2].Status!=Accepted {
		t.Errorf("wrong status history: %+v",events)
	}
	if !events[0].At.Before(events[1].At) || !events[1].At.Before(events[2].At) {
		t.Errorf("timestamps not ordered: %+v",events)
	}
}

// TestRepeatedStatusDoesNotDuplicateHistory verifies idempotency.
// Scenario: submit same status twice.
// Expected: only one historical change record.
//
// RU: Проверяет отсутствие повторных событий при неизменном статусе.
func TestRepeatedStatusDoesNotDuplicateHistory(t *testing.T) {
	repo,_,studentID,requirementID:=setupMaterialStatus(t)
	for i:=0;i<2;i++ {if _,err:=repo.Set(context.Background(),studentID,requirementID,UnderReview);err!=nil{t.Fatal(err)}}
	events,err:=repo.History(context.Background(),studentID,requirementID)
	if err!=nil || len(events)!=1 {t.Errorf("History=%+v, error=%v",events,err)}
}

// TestStatusRejectsUnknownDocumentsAndInvalidValues prevents mixing students
// and requirements and disallows invented states.
// RU: Проверяет отсутствие чужих документов и неизвестных статусов.
func TestStatusRejectsUnknownDocumentsAndInvalidValues(t *testing.T) {
	repo,_,studentID,requirementID:=setupMaterialStatus(t)
	for _,tc:=range []struct{student,doc int64;status Status;want error}{
		{0,requirementID,UnderReview,ErrInvalidScope},
		{studentID,9999,Accepted,ErrInvalidScope},
		{studentID,requirementID,Status("fake"),ErrInvalidStatus},
	}{
		if _,err:=repo.Set(context.Background(),tc.student,tc.doc,tc.status);!errors.Is(err,tc.want){
			t.Errorf("Set(%d,%d,%s)=%v, want %v",tc.student,tc.doc,tc.status,err,tc.want)
		}
	}
}

// TestStatusRespectsContextCancellation never changes status if the context
// was already canceled before the operation began.
// RU: Проверяет отмену изменения статуса до записи в SQLite.
func TestStatusRespectsContextCancellation(t *testing.T) {
	repo,_,studentID,requirementID:=setupMaterialStatus(t)
	ctx,cancel:=context.WithCancel(context.Background());cancel()
	if _,err:=repo.Set(ctx,studentID,requirementID,UnderReview);!errors.Is(err,context.Canceled) {
		t.Errorf("got %v, want context.Canceled",err)
	}
	entry,err:=repo.Get(context.Background(),studentID,requirementID)
	if err!=nil || entry.Status!=Missing {t.Errorf("current status after cancellation = %+v, %v",entry,err)}
}
