package profiles

import (
 "bytes"
 "context"
 "errors"
 "testing"
)
func TestPortableJSONRoundTripAndStrictValidation(t *testing.T){
 original:=sample()
 raw,err:=EncodeJSON(original);if err!=nil{t.Fatal(err)}
 if bytes.Contains(raw,[]byte("student_id"))||bytes.Contains(raw,[]byte("group_id")){
  t.Fatal("export leaks local student or group IDs")
 }
 parsed,err:=DecodeJSON(raw)
 if err!=nil||parsed.Name!=original.Name||len(parsed.Items)!=len(original.Items){t.Fatalf("%+v %v",parsed,err)}
 for _,invalid:=range [][]byte{
  []byte(`{"schema":9,"name":"Wrong","items":[{"key":"x","title":"A","position":0}]}`),
  []byte(`{"schema":1,"name":"Foreign","students":[],"items":[{"key":"x","title":"A","position":0}]}`),
  []byte(`{"schema":1,"name":"X","items":[{"key":"x","title":"Глава 1","position":0},{"key":"x","title":"Глава 2","position":1}]}`),
  append(raw,[]byte(" {}")...),
 }{
  if _,err:=DecodeJSON(invalid);!errors.Is(err,ErrInvalid){t.Fatalf("accepted unsafe json %q %v",invalid,err)}
 }
}
func TestJSONImportCreatesSeparateProfile(t *testing.T){
 s,_,_:=fixtures(t)
 one:=sample()
 data,err:=EncodeJSON(one);if err!=nil{t.Fatal(err)}
 imported,err:=s.ImportJSON(context.Background(),data)
 if err!=nil||imported.ID<=0||imported.Revision!=1{t.Fatalf("import %+v %v",imported,err)}
 if _,err:=s.ImportJSON(context.Background(),data);err==nil{t.Fatal("overwrote existing profile")}
}
func TestGroupApplicationHistoryIsIdempotent(t *testing.T){
 ctx:=context.Background()
 s,g,_:=fixtures(t)
 p,err:=s.Create(ctx,sample());if err!=nil{t.Fatal(err)}
 preview,err:=s.Preview(ctx,g,p.ID);if err!=nil{t.Fatal(err)}
 if _,err:=s.Apply(ctx,g,p.ID,preview.Token);err!=nil{t.Fatal(err)}
 repeat,err:=s.Preview(ctx,g,p.ID);if err!=nil{t.Fatal(err)}
 if _,err:=s.Apply(ctx,g,p.ID,repeat.Token);err!=nil{t.Fatal(err)}
 var changes int
 if err:=s.DB.QueryRowContext(ctx,"SELECT COUNT(*) FROM group_profile_applications WHERE group_id=?",g).Scan(&changes);err!=nil||changes!=1{t.Fatalf("repeat event %d %v",changes,err)}
}
