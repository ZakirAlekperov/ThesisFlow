package integrity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
)

// TestAuditDetectsMissingCorruptAndIntactFiles checks the immutable version
// metadata against actual files instead of trusting the database alone.
// RU: Проверяет реальные файлы по сохранённым SHA-256 и размерам.
func TestAuditDetectsMissingCorruptAndIntactFiles(t *testing.T) {
	ctx:=context.Background()
	db,err:=sqlite.Open(ctx,filepath.Join(t.TempDir(),"test.sqlite"))
	if err!=nil{t.Fatal(err)}
	defer db.Close()
	root:=filepath.Join(t.TempDir(),"storage")
	if err:=os.MkdirAll(root,0700);err!=nil{t.Fatal(err)}
	if _,err:=db.Exec(`CREATE TABLE file_versions (
		id INTEGER PRIMARY KEY, relative_path TEXT NOT NULL,
		file_size INTEGER NOT NULL, sha256 TEXT NOT NULL)`);err!=nil{t.Fatal(err)}
	add:=func(id int,name string,expected []byte, actual []byte,exists bool) {
		t.Helper()
		digest:=sha256.Sum256(expected)
		if _,err:=db.Exec("INSERT INTO file_versions(id,relative_path,file_size,sha256) VALUES(?,?,?,?)",id,name,len(expected),hex.EncodeToString(digest[:]));err!=nil{t.Fatal(err)}
		if exists {
			if err:=os.WriteFile(filepath.Join(root,name),actual,0600);err!=nil{t.Fatal(err)}
		}
	}
	add(1,"ok.docx",[]byte("intact"),[]byte("intact"),true)
	add(2,"missing.docx",[]byte("missing"),nil,false)
	add(3,"tampered.docx",[]byte("original"),[]byte("tampered"),true)
	report,err:=Check(ctx,db,root)
	if err!=nil{t.Fatal(err)}
	if report.Checked!=3 || report.Intact!=1 || report.Missing!=1 || report.Corrupt!=1 {
		t.Errorf("audit = %+v",report)
	}
	if len(report.Issues)!=2 {t.Errorf("issues = %+v",report.Issues)}
}

// TestAuditRejectsEscapingAndSymlinkReferences checks that a malicious
// database path cannot read files outside of the managed workspace.
// RU: Проверяет защиту от выхода за пределы каталога и символьных ссылок.
func TestAuditRejectsEscapingAndSymlinkReferences(t *testing.T) {
	ctx:=context.Background()
	db,err:=sqlite.Open(ctx,filepath.Join(t.TempDir(),"test.sqlite"))
	if err!=nil{t.Fatal(err)}
	defer db.Close()
	root:=filepath.Join(t.TempDir(),"storage")
	if err:=os.MkdirAll(root,0700);err!=nil{t.Fatal(err)}
	if _,err:=db.Exec("CREATE TABLE file_versions(id INTEGER PRIMARY KEY,relative_path TEXT,file_size INTEGER,sha256 TEXT)");err!=nil{t.Fatal(err)}
	digest:=sha256.Sum256([]byte("secret"))
	for i,path:=range []string{"../outside.txt","link.txt"} {
		if _,err:=db.Exec("INSERT INTO file_versions(id,relative_path,file_size,sha256) VALUES(?,?,?,?)",i+1,path,6,hex.EncodeToString(digest[:]));err!=nil{t.Fatal(err)}
	}
	outside:=filepath.Join(t.TempDir(),"secret.txt")
	if err:=os.WriteFile(outside,[]byte("secret"),0600);err!=nil{t.Fatal(err)}
	if err:=os.Symlink(outside,filepath.Join(root,"link.txt"));err!=nil{t.Skipf("symlinks unsupported: %v",err)}
	report,err:=Check(ctx,db,root)
	if err!=nil{t.Fatal(err)}
	if report.Unsafe!=2 {t.Errorf("unsafe = %+v",report)}
}

// TestAuditDoesNotWriteRecords proves an integrity scan is read-only.
// RU: Проверяет, что проверка целостности не изменяет данные SQLite.
func TestAuditDoesNotWriteRecords(t *testing.T) {
	ctx:=context.Background()
	db,err:=sqlite.Open(ctx,filepath.Join(t.TempDir(),"test.sqlite"))
	if err!=nil{t.Fatal(err)}
	defer db.Close()
	if _,err:=db.Exec("CREATE TABLE file_versions(id INTEGER PRIMARY KEY,relative_path TEXT,file_size INTEGER,sha256 TEXT)");err!=nil{t.Fatal(err)}
	if _,err:=Check(ctx,db,t.TempDir());err!=nil{t.Fatal(err)}
	var count int
	if err:=db.QueryRow("SELECT count(*) FROM file_versions").Scan(&count);err!=nil{t.Fatal(err)}
	if count!=0{t.Fatal("audit changed database")}
}
