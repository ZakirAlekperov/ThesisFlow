// Package profiles manages immutable reusable document profiles and
// transactional group application without duplicating student requirements.
// Пакет profiles хранит ревизии шаблонов обязательных документов ВКР.
package profiles

import (
 "context"
 "crypto/sha256"
 "database/sql"
 "encoding/hex"
 "encoding/json"
 "errors"
 "fmt"
 "strings"
)

var (
 ErrInvalid=errors.New("invalid requirement profile")
 ErrConflict=errors.New("profile conflicts with group requirements")
 ErrStale=errors.New("profile preview is outdated")
)
type Item struct {
 Key string `json:"key"`
 Title string `json:"title"`
 Description string `json:"description,omitempty"`
 Position int `json:"position"`
}
type Profile struct {
 ID int64 `json:"-"`
 Name string `json:"name"`
 Description string `json:"description,omitempty"`
 RevisionID int64 `json:"-"`
 Revision int `json:"revision"`
 Items []Item `json:"items"`
}
type Action struct {Key,Title,Kind string; RequirementID int64}
type Plan struct {
 GroupID,ProfileID,RevisionID int64
 Revision int
 Token string
 Actions []Action
 Conflict bool
}
type Service struct{DB *sql.DB}
func New(db *sql.DB)*Service{return &Service{DB:db}}

// ValidateItems rejects duplicates, empty fields and overlong names before
// any mutation. All profile items are mandatory in 0.7.
// RU: Проверяем дубликаты/длину и не вводим ложную опциональность.
func ValidateItems(items []Item)error{
 if len(items)==0||len(items)>60{return ErrInvalid}
 seen:=map[string]bool{}
 keys:=map[string]bool{}
 for i,x:=range items{
  title:=strings.TrimSpace(x.Title)
  key:=strings.TrimSpace(x.Key)
  if title==""||len([]rune(title))>120||len([]rune(x.Description))>500||
   len(key)==0||len(key)>60||x.Position!=i||seen[strings.ToLower(title)]||keys[key]{
   return ErrInvalid
  }
  seen[strings.ToLower(title)]=true;keys[key]=true
 }
 return nil
}
func validateProfile(p Profile)error{
 if strings.TrimSpace(p.Name)==""||len([]rune(p.Name))>120||len([]rune(p.Description))>800{return ErrInvalid}
 return ValidateItems(p.Items)
}
func insertRevision(ctx context.Context,tx *sql.Tx,profileID int64,revision int,items []Item)(int64,error){
 result,err:=tx.ExecContext(ctx,"INSERT INTO profile_revisions(profile_id,number) VALUES(?,?)",profileID,revision)
 if err!=nil{return 0,err}
 id,err:=result.LastInsertId()
 if err!=nil{return 0,err}
 for _,x:=range items{
  _,err=tx.ExecContext(ctx,`INSERT INTO profile_items(revision_id,item_key,title,description,position)
    VALUES(?,?,?,?,?)`,id,x.Key,strings.TrimSpace(x.Title),x.Description,x.Position)
  if err!=nil{return 0,err}
 }
 return id,nil
}
// Create adds one reusable profile and its immutable revision atomically.
// RU: Создание профиля и первой ревизии — одна транзакция.
func(s *Service)Create(ctx context.Context,p Profile)(Profile,error){
 if err:=validateProfile(p);err!=nil{return Profile{},err}
 tx,err:=s.DB.BeginTx(ctx,nil);if err!=nil{return Profile{},err};defer tx.Rollback()
 result,err:=tx.ExecContext(ctx,"INSERT INTO requirement_profiles(name,description) VALUES(?,?)",strings.TrimSpace(p.Name),p.Description)
 if err!=nil{return Profile{},fmt.Errorf("create profile: %w",err)}
 p.ID,err=result.LastInsertId();if err!=nil{return Profile{},err}
 p.Revision=1
 p.RevisionID,err=insertRevision(ctx,tx,p.ID,1,p.Items);if err!=nil{return Profile{},err}
 if err:=tx.Commit();err!=nil{return Profile{},err}
 return p,nil
}
// Revise appends a complete immutable revision. It never changes groups.
// RU: Изменение профиля не меняет уже идущие ВКР автоматически.
func(s *Service)Revise(ctx context.Context,profileID int64,expectedRevision int,items []Item)(Profile,error){
 if err:=ValidateItems(items);err!=nil{return Profile{},err}
 tx,err:=s.DB.BeginTx(ctx,nil);if err!=nil{return Profile{},err};defer tx.Rollback()
 var name,description string
 err=tx.QueryRowContext(ctx,"SELECT name,description FROM requirement_profiles WHERE id=?",profileID).Scan(&name,&description)
 if errors.Is(err,sql.ErrNoRows){return Profile{},ErrInvalid};if err!=nil{return Profile{},err}
 var latest int
 if err:=tx.QueryRowContext(ctx,"SELECT COALESCE(MAX(number),0) FROM profile_revisions WHERE profile_id=?",profileID).Scan(&latest);err!=nil{return Profile{},err}
 if latest!=expectedRevision{return Profile{},ErrStale}
 newID,err:=insertRevision(ctx,tx,profileID,latest+1,items)
 if err!=nil{return Profile{},err}
 if err:=tx.Commit();err!=nil{return Profile{},err}
 return Profile{ID:profileID,Name:name,Description:description,Revision:latest+1,RevisionID:newID,Items:items},nil
}
func(s *Service)Get(ctx context.Context,id int64)(Profile,error){
 var p Profile
 p.ID=id
 err:=s.DB.QueryRowContext(ctx,`SELECT name,description FROM requirement_profiles WHERE id=?`,id).Scan(&p.Name,&p.Description)
 if errors.Is(err,sql.ErrNoRows){return Profile{},ErrInvalid};if err!=nil{return Profile{},err}
 err=s.DB.QueryRowContext(ctx,`SELECT id,number FROM profile_revisions WHERE profile_id=? ORDER BY number DESC LIMIT 1`,id).Scan(&p.RevisionID,&p.Revision)
 if err!=nil{return Profile{},err}
 rows,err:=s.DB.QueryContext(ctx,"SELECT item_key,title,description,position FROM profile_items WHERE revision_id=? ORDER BY position",p.RevisionID)
 if err!=nil{return Profile{},err};defer rows.Close()
 for rows.Next(){
  var item Item
  if err:=rows.Scan(&item.Key,&item.Title,&item.Description,&item.Position);err!=nil{return Profile{},err}
  p.Items=append(p.Items,item)
 }
 return p,rows.Err()
}
func(s *Service)List(ctx context.Context)([]Profile,error){
 rows,err:=s.DB.QueryContext(ctx,`SELECT p.id,p.name,p.description,r.id,r.number
 FROM requirement_profiles p JOIN profile_revisions r ON r.id=(
 SELECT id FROM profile_revisions WHERE profile_id=p.id ORDER BY number DESC LIMIT 1)
 ORDER BY p.name,p.id`)
 if err!=nil{return nil,err};defer rows.Close()
 result:=[]Profile{}
 for rows.Next(){
  var p Profile
  if err:=rows.Scan(&p.ID,&p.Name,&p.Description,&p.RevisionID,&p.Revision);err!=nil{return nil,err}
  result=append(result,p)
 }
 return result,rows.Err()
}
type rowQuerier interface{QueryContext(context.Context,string,...any)(*sql.Rows,error)}
func makePlan(ctx context.Context,q rowQuerier,groupID int64,p Profile)(Plan,error){
 plan:=Plan{GroupID:groupID,ProfileID:p.ID,RevisionID:p.RevisionID,Revision:p.Revision,Actions:[]Action{}}
 rows,err:=q.QueryContext(ctx,`SELECT r.id,r.title,r.due_date,COALESCE(b.item_key,'')
 FROM requirements r LEFT JOIN group_profile_bindings b ON b.requirement_id=r.id
 WHERE r.group_id=? AND r.student_id IS NULL ORDER BY r.id`,groupID)
 if err!=nil{return Plan{},err}
 type existing struct{ID int64;Title,Due,Key string}
 byKey:=map[string]existing{};byTitle:=map[string]existing{}
 for rows.Next(){
  var x existing
  if err:=rows.Scan(&x.ID,&x.Title,&x.Due,&x.Key);err!=nil{rows.Close();return Plan{},err}
  if x.Key!=""{byKey[x.Key]=x}
  byTitle[strings.ToLower(x.Title)]=x
 }
 err=rows.Err();rows.Close()
 if err!=nil{return Plan{},err}
 for _,item:=range p.Items{
  x,found:=byKey[item.Key]
  action:=Action{Key:item.Key,Title:item.Title}
  if found{
   action.RequirementID=x.ID
   if x.Title==item.Title{action.Kind="unchanged"}else{action.Kind="conflict";plan.Conflict=true}
  }else if dup,ok:=byTitle[strings.ToLower(item.Title)];ok{
   action.Kind="conflict";action.RequirementID=dup.ID;plan.Conflict=true
  }else{action.Kind="add"}
  plan.Actions=append(plan.Actions,action)
 }
 // Signature covers current group requirements and all bound keys, so a
 // stale preview cannot overwrite concurrent edits or reuse a changed map.
 // RU: Подпись включает состояние группы и номер ревизии.
 snapshot:=struct{Plan []Action;Existing map[string]existing;Revision int64}{
  Plan:plan.Actions,Existing:byTitle,Revision:p.RevisionID,
 }
 blob,err:=json.Marshal(snapshot);if err!=nil{return Plan{},err}
 sum:=sha256.Sum256(blob);plan.Token=hex.EncodeToString(sum[:])
 return plan,nil
}
// Preview computes an immutable diff, and exposes no writes.
// RU: Предпросмотр не создаёт требований в группе.
func(s *Service)Preview(ctx context.Context,groupID,profileID int64)(Plan,error){
 p,err:=s.Get(ctx,profileID);if err!=nil{return Plan{},err}
 var exists int
 if err:=s.DB.QueryRowContext(ctx,"SELECT EXISTS(SELECT 1 FROM student_groups WHERE id=? AND archived_at IS NULL)",groupID).Scan(&exists);err!=nil{return Plan{},err}
 if exists!=1{return Plan{},ErrInvalid}
 return makePlan(ctx,s.DB,groupID,p)
}
// Apply rechecks the preview under a SQLite write lock and inserts only new
// group requirements. Existing document IDs and histories are untouched.
// RU: Применение атомарно и повторно безопасно, но конфликты блокируют запись.
func(s *Service)Apply(ctx context.Context,groupID,profileID int64,token string)(Plan,error){
 if len(token)!=64{return Plan{},ErrStale}
 p,err:=s.Get(ctx,profileID);if err!=nil{return Plan{},err}
 // SQLite BEGIN IMMEDIATE is deliberately acquired before checking the
 // snapshot; a deferred transaction permits two simultaneous applies.
 conn,err:=s.DB.Conn(ctx);if err!=nil{return Plan{},err};defer conn.Close()
 if _,err:=conn.ExecContext(ctx,"BEGIN IMMEDIATE");err!=nil{return Plan{},err}
 committed:=false;defer func(){if !committed{_,_=conn.ExecContext(context.Background(),"ROLLBACK")}}()
 plan,err:=makePlan(ctx,conn,groupID,p)
 if err!=nil{return Plan{},err}
 if plan.Token!=token{return Plan{},ErrStale}
 if plan.Conflict{return Plan{},ErrConflict}
 // A group cannot silently switch to an entirely different profile.
 var otherID int64
 err=conn.QueryRowContext(ctx,"SELECT profile_id FROM group_profile_state WHERE group_id=?",groupID).Scan(&otherID)
 if err!=nil&&!errors.Is(err,sql.ErrNoRows){return Plan{},err}
 if err==nil&&otherID!=profileID{return Plan{},ErrConflict}
 for _,action:=range plan.Actions{
  if action.Kind!="add"{continue}
  res,err:=conn.ExecContext(ctx,`INSERT INTO requirements(group_id,student_id,title,title_key,due_date)
  VALUES(?,NULL,?,?, '')`,groupID,action.Title,strings.ToLower(action.Title))
  if err!=nil{return Plan{},fmt.Errorf("apply requirement: %w",err)}
  id,err:=res.LastInsertId();if err!=nil{return Plan{},err}
  if _,err:=conn.ExecContext(ctx,`INSERT INTO group_profile_bindings(group_id,profile_id,item_key,requirement_id)
   VALUES(?,?,?,?)`,groupID,profileID,action.Key,id);err!=nil{return Plan{},err}
 }
 if _,err:=conn.ExecContext(ctx,`INSERT INTO group_profile_state(group_id,profile_id,revision_id)
 VALUES(?,?,?) ON CONFLICT(group_id) DO UPDATE SET revision_id=excluded.revision_id,applied_at=datetime('now')`,groupID,profileID,p.RevisionID);err!=nil{return Plan{},err}
 if _,err:=conn.ExecContext(ctx,"COMMIT");err!=nil{return Plan{},err}
 committed=true
 return plan,nil
}
