package backup

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// BrowserService exposes backed-up bytes and isolated recovery to the local
// teacher UI. It never changes the active workspace during restoration.
// RU: BrowserService позволяет скачать ZIP и восстановить его только отдельно.
type BrowserService struct {
	DB *sql.DB
	Workspace string
}

// Export creates a snapshot in a private temporary location then streams ZIP
// bytes to the caller, deleting temporary data on return.
// RU: Export создаёт временную копию и передаёт ZIP без сохранения лишних файлов.
func (s *BrowserService) Export(ctx context.Context, destination io.Writer) error {
	if err:=ctx.Err();err!=nil{return err}
	dir,err:=os.MkdirTemp("","thesisflow-web-backup-*")
	if err!=nil{return fmt.Errorf("create backup temp directory: %w",err)}
	defer os.RemoveAll(dir)
	archivePath:=filepath.Join(dir,"ThesisFlow.zip")
	if err:=Export(ctx,s.DB,filepath.Join(s.Workspace,"storage"),archivePath);err!=nil{
		return err
	}
	f,err:=os.Open(archivePath)
	if err!=nil{return err}
	defer f.Close()
	if _,err:=io.Copy(destination,f);err!=nil{return fmt.Errorf("stream snapshot: %w",err)}
	return nil
}

// Restore writes the archive to a bounded temporary file and restores it to a
// cryptographically unique new directory under the local workspace's parent.
// It never switches or overwrites the active SQLite connection.
//
// RU: Restore проверяет ZIP, восстанавливает только в новый каталог рядом
// с рабочей базой и не переключает действующую базу приложения.
func (s *BrowserService) Restore(ctx context.Context, archive io.Reader) (string,error) {
	if err:=ctx.Err();err!=nil{return "",err}
	tmp,err:=os.CreateTemp("","thesisflow-web-recovery-*.zip")
	if err!=nil{return "",fmt.Errorf("stage recovery archive: %w",err)}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	count,err:=io.Copy(tmp,io.LimitReader(archive,MaxArchiveBytes+1))
	if err!=nil{return "",fmt.Errorf("read recovery upload: %w",err)}
	if count>MaxArchiveBytes{return "",ErrTooLarge}
	if err:=tmp.Close();err!=nil{return "",err}
	suffix:=make([]byte,8)
	if _,err:=rand.Read(suffix);err!=nil{return "",err}
	target:=filepath.Join(filepath.Dir(s.Workspace),
		"ThesisFlow-restored-"+time.Now().Format("20060102")+"-"+hex.EncodeToString(suffix))
	if err:=Restore(tmp.Name(),target);err!=nil{return "",err}
	return target,nil
}
