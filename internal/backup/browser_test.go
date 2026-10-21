package backup

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
)

// TestBrowserServiceRoundTrip uses real SQLite and document bytes to check
// browser backup downloading and safe separate-folder restoration.
// RU: Проверяет скачивание ZIP и восстановление в новую папку из браузера.
func TestBrowserServiceRoundTrip(t *testing.T) {
	ctx:=context.Background()
	root:=t.TempDir()
	workspace:=filepath.Join(root,"ThesisFlow")
	db,err:=sqlite.Open(ctx,filepath.Join(workspace,"thesisflow.sqlite"))
	if err!=nil{t.Fatal(err)}
	defer db.Close()
	if _,err:=db.Exec("CREATE TABLE probe (value TEXT)");err!=nil{t.Fatal(err)}
	if _,err:=db.Exec("INSERT INTO probe(value) VALUES (?)","real data");err!=nil{t.Fatal(err)}
	if err:=os.MkdirAll(filepath.Join(workspace,"storage","accepted"),0700);err!=nil{t.Fatal(err)}
	if err:=os.WriteFile(filepath.Join(workspace,"storage","accepted","Глава 1.docx"),[]byte("chapter document"),0600);err!=nil{t.Fatal(err)}
	service:=&BrowserService{DB:db,Workspace:workspace}
	var data bytes.Buffer
	if err:=service.Export(ctx,&data);err!=nil{t.Fatalf("Export: %v",err)}
	if data.Len()<128{t.Fatalf("empty ZIP: %d",data.Len())}
	restored,err:=service.Restore(ctx,bytes.NewReader(data.Bytes()))
	if err!=nil{t.Fatalf("Restore: %v",err)}
	if !filepath.IsLocal(filepath.Base(restored)) || restored==workspace {t.Fatalf("unsafe restore destination: %q",restored)}
	disk,err:=sqlite.Open(ctx,filepath.Join(restored,"thesisflow.sqlite"))
	if err!=nil{t.Fatal(err)}
	defer disk.Close()
	var value string
	if err:=disk.QueryRow("SELECT value FROM probe").Scan(&value);err!=nil{t.Fatal(err)}
	if value!="real data"{t.Fatalf("restored database value: %q",value)}
	file,err:=os.ReadFile(filepath.Join(restored,"storage","accepted","Глава 1.docx"))
	if err!=nil{t.Fatal(err)}
	if string(file)!="chapter document"{t.Fatalf("restored file: %q",file)}
}

// TestBrowserRestoreRejectsInvalidArchive preserves current workspace.
// RU: При повреждённом ZIP рабочая база не перезаписывается.
func TestBrowserRestoreRejectsInvalidArchive(t *testing.T) {
	ctx:=context.Background()
	workspace:=filepath.Join(t.TempDir(),"ThesisFlow")
	if err:=os.MkdirAll(workspace,0700);err!=nil{t.Fatal(err)}
	if err:=os.WriteFile(filepath.Join(workspace,"thesisflow.sqlite"),[]byte("do not replace"),0600);err!=nil{t.Fatal(err)}
	service:=&BrowserService{Workspace:workspace}
	if _,err:=service.Restore(ctx,bytes.NewReader([]byte("invalid ZIP")));err==nil{t.Fatal("Restore accepted invalid archive")}
	value,err:=os.ReadFile(filepath.Join(workspace,"thesisflow.sqlite"))
	if err!=nil{t.Fatal(err)}
	if string(value)!="do not replace"{t.Fatalf("workspace modified: %q",value)}
}
