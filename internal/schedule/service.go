// Package schedule provides group-wide deadlines and explicit student
// exceptions. Status events and original submission times remain immutable.
// Пакет schedule управляет сроками, не меняя события сдачи.
package schedule

import (
 "context"
 "crypto/sha256"
 "database/sql"
 "encoding/hex"
 "encoding/json"
 "errors"
 "fmt"
 "strings"
 "time"
)
var ErrInvalid=errors.New("invalid schedule data")
var ErrStale=errors.New("schedule preview is outdated")
type Deadline struct{RequirementID int64;Title,Previous,Next string}
type CalendarEntry struct{
 GroupID,StudentID,RequirementID int64
 GroupName,StudentName,Title,DueDate,Status,Category string
 OverdueDays int
}
type Service struct{DB *sql.DB;Now func()time.Time}
func New(db *sql.DB)*Service{return &Service{DB:db,Now:time.Now}}
func dateValid(v string)bool{
 if v==""{return true}
 t,err:=time.Parse("2006-01-02",v);return err==nil&&t.Format("2006-01-02")==v
}
func(s *Service)GroupDeadlines(ctx context.Context,groupID int64)([]Deadline,error){
 rows,err:=s.DB.QueryContext(ctx,`SELECT id,title,due_date FROM requirements
 WHERE group_id=? AND student_id IS NULL ORDER BY title_key,id`,groupID)
 if err!=nil{return nil,err};defer rows.Close()
 result:=[]Deadline{}
 for rows.Next(){var x Deadline
  if err:=rows.Scan(&x.RequirementID,&x.Title,&x.Previous);err!=nil{return nil,err}
  x.Next=x.Previous;result=append(result,x)
 }
 return result,rows.Err()
}
func tokenFor(groupID int64,rows []Deadline)string{
 data,_:=json.Marshal(struct{ID int64;Rows []Deadline}{groupID,rows})
 sum:=sha256.Sum256(data);return hex.EncodeToString(sum[:])
}
// Preview validates the exact set of current group requirement IDs.
// RU: Ведомость сроков предварительно сверяется с существующими ID.
func(s *Service)Preview(ctx context.Context,groupID int64,proposed map[int64]string)([]Deadline,string,error){
 current,err:=s.GroupDeadlines(ctx,groupID);if err!=nil{return nil,"",err}
 for id,v:=range proposed{if id<=0||!dateValid(v){return nil,"",ErrInvalid}}
 seen:=map[int64]bool{}
 for i:=range current{
  row:=&current[i];seen[row.RequirementID]=true
  if val,exists:=proposed[row.RequirementID];exists{row.Next=val}
 }
 for id:=range proposed{if !seen[id]{return nil,"",ErrInvalid}}
 return current,tokenFor(groupID,current),""
}
// ApplyDeadlines compares the preview inside a write transaction and records
// independent schedule events, not material submission events.
// RU: Меняем сроки атомарно, логируем старое/новое без изменения сдачи.
func(s *Service)ApplyDeadlines(ctx context.Context,groupID int64,proposed map[int64]string,expected string)error{
 if len(expected)!=64{return ErrStale}
 conn,err:=s.DB.Conn(ctx);if err!=nil{return err};defer conn.Close()
 if _,err:=conn.ExecContext(ctx,"BEGIN IMMEDIATE");err!=nil{return err}
 done:=false;defer func(){if !done{_,_=conn.ExecContext(context.Background(),"ROLLBACK")}}()
 rows,err:=conn.QueryContext(ctx,`SELECT id,title,due_date FROM requirements WHERE group_id=? AND student_id IS NULL ORDER BY title_key,id`,groupID)
 if err!=nil{return err}
 current:=[]Deadline{};seen:=map[int64]bool{}
 for rows.Next(){var x Deadline
  if err:=rows.Scan(&x.RequirementID,&x.Title,&x.Previous);err!=nil{rows.Close();return err}
  x.Next=x.Previous;seen[x.RequirementID]=true
  if next,ok:=proposed[x.RequirementID];ok{if !dateValid(next){rows.Close();return ErrInvalid};x.Next=next}
  current=append(current,x)
 }
 if err:=rows.Err();err!=nil{rows.Close();return err};rows.Close()
 for id:=range proposed{if !seen[id]{return ErrInvalid}}
 if tokenFor(groupID,current)!=expected{return ErrStale}
 for _,x:=range current{
  if x.Next==x.Previous{continue}
  if _,err:=conn.ExecContext(ctx,`UPDATE requirements SET due_date=? WHERE id=? AND group_id=?`,x.Next,x.RequirementID,groupID);err!=nil{return err}
  if _,err:=conn.ExecContext(ctx,`INSERT INTO requirement_changes(group_id,requirement_id,old_value,new_value,kind)
   VALUES(?,?,?,?,'group_deadline')`,groupID,x.RequirementID,x.Previous,x.Next);err!=nil{return err}
 }
 if _,err:=conn.ExecContext(ctx,"COMMIT");err!=nil{return err}
 done=true;return nil
}
// SetOverride supports 'inherit', 'none', and an explicit ISO date.
// RU: Индивидуальный срок может наследоваться, быть отменён или задан.
func(s *Service)SetOverride(ctx context.Context,groupID,studentID,requirementID int64,kind,value string)error{
 if groupID<=0||studentID<=0||requirementID<=0{return ErrInvalid}
 if kind!="inherit"&&kind!="none"&&kind!="date"{return ErrInvalid}
 if kind=="date"&&(!dateValid(value)||value==""){return ErrInvalid}
 if kind!="date"{value=""}
 tx,err:=s.DB.BeginTx(ctx,nil);if err!=nil{return err};defer tx.Rollback()
 var valid int
 if err:=tx.QueryRowContext(ctx,`SELECT EXISTS(SELECT 1 FROM students s
 JOIN requirements r ON r.group_id=s.group_id AND r.id=?
 WHERE s.id=? AND s.group_id=? AND (r.student_id IS NULL OR r.student_id=s.id))`,
  requirementID,studentID,groupID).Scan(&valid);err!=nil{return err}
 if valid!=1{return ErrInvalid}
 var previousKind,previousDate string
 err=tx.QueryRowContext(ctx,"SELECT kind,due_date FROM due_overrides WHERE student_id=? AND requirement_id=?",studentID,requirementID).Scan(&previousKind,&previousDate)
 if err!=nil&&!errors.Is(err,sql.ErrNoRows){return err}
 if err==sql.ErrNoRows{previousKind="inherit"}
 if previousKind==kind&&previousDate==value{return nil}
 if kind=="inherit"{
  _,err=tx.ExecContext(ctx,"DELETE FROM due_overrides WHERE student_id=? AND requirement_id=?",studentID,requirementID)
 }else{
  _,err=tx.ExecContext(ctx,`INSERT INTO due_overrides(student_id,requirement_id,kind,due_date)
  VALUES(?,?,?,?) ON CONFLICT(student_id,requirement_id) DO UPDATE SET kind=excluded.kind,due_date=excluded.due_date`,studentID,requirementID,kind,value)
 }
 if err!=nil{return err}
 _,err=tx.ExecContext(ctx,`INSERT INTO requirement_changes(group_id,requirement_id,student_id,old_value,new_value,kind)
 VALUES(?,?,?,?,?,'student_deadline')`,groupID,requirementID,studentID,previousKind+":"+previousDate,kind+":"+value)
 if err!=nil{return err}
 return tx.Commit()
}
// Calendar reads actual current material states and effective deadlines in
// one join, excluding archived students and groups.
// RU: Календарь не реконструирует прошлую готовность.
func(s *Service)Calendar(ctx context.Context,groupID int64)([]CalendarEntry,error){
 if groupID<0{return nil,ErrInvalid}
 rows,err:=s.DB.QueryContext(ctx,`SELECT g.id,s.id,r.id,g.name,s.full_name,r.title,
 CASE WHEN o.kind='none' THEN '' WHEN o.kind='date' THEN o.due_date ELSE r.due_date END,
 COALESCE(m.status,'missing')
 FROM student_groups g JOIN students s ON s.group_id=g.id AND s.archived_at IS NULL
 JOIN requirements r ON r.group_id=g.id AND (r.student_id IS NULL OR r.student_id=s.id)
 LEFT JOIN due_overrides o ON o.student_id=s.id AND o.requirement_id=r.id
 LEFT JOIN material_status m ON m.student_id=s.id AND m.requirement_id=r.id
 WHERE g.archived_at IS NULL AND (?=0 OR g.id=?)
 ORDER BY g.name,s.full_name,r.title_key`,groupID,groupID)
 if err!=nil{return nil,err};defer rows.Close()
 result:=[]CalendarEntry{}
 today:=s.Now().In(time.Local)
 y,m,d:=today.Date();now:=time.Date(y,m,d,0,0,0,0,time.UTC)
 for rows.Next(){
  var e CalendarEntry
  if err:=rows.Scan(&e.GroupID,&e.StudentID,&e.RequirementID,&e.GroupName,&e.StudentName,&e.Title,&e.DueDate,&e.Status);err!=nil{return nil,err}
  if e.DueDate==""{e.Category="без срока"}else{
   deadline,err:=time.Parse("2006-01-02",e.DueDate)
   if err!=nil{return nil,fmt.Errorf("invalid stored deadline: %w",err)}
   days:=int(deadline.Sub(now).Hours()/24)
   if e.Status=="accepted"{e.Category="принято"}else if days<0{e.Category="просрочено";e.OverdueDays=-days}else if days==0{e.Category="сегодня"}else if days<=7{e.Category="7 дней"}else{e.Category="позже"}
  }
  result=append(result,e)
 }
 return result,rows.Err()
}
var _=strings.TrimSpace
