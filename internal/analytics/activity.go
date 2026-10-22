package analytics

import (
 "context"
 "fmt"
 "sort"
 "time"
)

// Week is a factual count of file submissions, acceptance decisions and
// revision requests in one local Monday-based calendar week.
// RU: Week учитывает события, а не выдуманное прошлое состояние готовности.
type Week struct {
 Start string
 Uploaded int
 Accepted int
 Returned int
}

// Activity returns persisted events by local calendar week in inclusive date
// range [from,to]. It never reconstructs historical percentages.
// RU: Activity считает события за период без изменения БД.
func(r *Repository)Activity(ctx context.Context,from,to time.Time,groupID int64)([]Week,error){
 if err:=ctx.Err();err!=nil{return nil,err}
 if groupID<0||to.Before(from)||to.Sub(from)>366*24*time.Hour{return nil,ErrInvalidFilter}
 const query=`SELECT v.uploaded_at,'uploaded' FROM file_versions v
 JOIN students s ON s.id=v.student_id JOIN student_groups g ON g.id=s.group_id
 WHERE s.archived_at IS NULL AND g.archived_at IS NULL AND (?=0 OR g.id=?)
 UNION ALL SELECT e.occurred_at,'accepted' FROM material_events e
 JOIN students s ON s.id=e.student_id JOIN student_groups g ON g.id=s.group_id
 WHERE e.status='accepted' AND s.archived_at IS NULL AND g.archived_at IS NULL AND (?=0 OR g.id=?)
 UNION ALL SELECT f.created_at,'returned' FROM review_feedback f
 JOIN students s ON s.id=f.student_id JOIN student_groups g ON g.id=s.group_id
 WHERE f.kind='revision' AND s.archived_at IS NULL AND g.archived_at IS NULL AND (?=0 OR g.id=?)`
 rows,err:=r.DB.QueryContext(ctx,query,groupID,groupID,groupID,groupID,groupID,groupID)
 if err!=nil{return nil,fmt.Errorf("read activity events: %w",err)}
 defer rows.Close()
 zone:=from.Location()
 begin:=from.In(zone).Format("2006-01-02")
 end:=to.In(zone).Format("2006-01-02")
 counts:=map[string]*Week{}
 for rows.Next(){
  var stamp,kind string
  if err:=rows.Scan(&stamp,&kind);err!=nil{return nil,fmt.Errorf("scan activity: %w",err)}
  at,err:=time.Parse(time.RFC3339Nano,stamp)
  if err!=nil{return nil,fmt.Errorf("invalid event time: %w",err)}
  local:=at.In(zone)
  day:=local.Format("2006-01-02")
  if day<begin||day>end{continue}
  local=time.Date(local.Year(),local.Month(),local.Day(),0,0,0,0,zone)
  monday:=local.AddDate(0,0,-(int(local.Weekday())+6)%7).Format("2006-01-02")
  week:=counts[monday]
  if week==nil{week=&Week{Start:monday};counts[monday]=week}
  switch kind{case "uploaded":week.Uploaded++;case "accepted":week.Accepted++;case "returned":week.Returned++}
 }
 if err:=rows.Err();err!=nil{return nil,fmt.Errorf("iterate activity: %w",err)}
 keys:=make([]string,0,len(counts))
 for key:=range counts{keys=append(keys,key)}
 sort.Strings(keys)
 result:=make([]Week,0,len(keys))
 for _,key:=range keys{result=append(result,*counts[key])}
 return result,nil
}
