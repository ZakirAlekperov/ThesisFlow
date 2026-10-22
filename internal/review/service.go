// Package review accepts or rejects submitted document versions and maintains
// a human-readable projection of the latest approved files.
// Пакет review принимает и отклоняет версии документов и поддерживает
// человекочитаемую папку последних принятых файлов.
package review

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/ZakirAlekperov/ThesisFlow/internal/fileversions"
	"github.com/ZakirAlekperov/ThesisFlow/internal/materials"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storagepath"
)

// ErrInvalidVersion means that a version does not belong to the requested
// student and requirement, or does not exist.
// RU: ErrInvalidVersion означает отсутствующую или чужую версию файла.
var ErrInvalidVersion = errors.New("document version is not in the selected scope")

// Accepted describes a committed human-readable approved-file projection.
// RU: Accepted — зафиксированная принятая версия с понятным путём.
type Accepted struct {
	StudentID     int64
	RequirementID int64
	VersionID     int64
	RelativePath  string
}

// Service controls approval and the accepted filesystem projection.
// RU: Service управляет приёмкой и пользовательскими папками документов.
type Service struct {
	DB       *sql.DB
	Versions *fileversions.Store
	Clock    func() time.Time
	// interruptApproval is a deterministic crash-injection hook for package tests.
	// RU: Используется только в тестах аварийных точек приёмки.
	interruptApproval func(stage string) error
}

// NewService uses an existing database and version store.
// RU: NewService работает на общей SQLite и хранилище версий.
func NewService(db *sql.DB, versions *fileversions.Store) *Service {
	return &Service{DB: db, Versions: versions, Clock: time.Now}
}

// Init creates an index of currently accepted files. The accepted user folder
// is intentionally not part of the internal versions directory.
// RU: Init создаёт индекс принятых документов отдельно от истории версий.
func (s *Service) Init(ctx context.Context) error {
	const query = `CREATE TABLE IF NOT EXISTS accepted_files (
		student_id INTEGER NOT NULL REFERENCES students(id) ON DELETE RESTRICT,
		requirement_id INTEGER NOT NULL REFERENCES requirements(id) ON DELETE RESTRICT,
		version_id INTEGER NOT NULL REFERENCES file_versions(id) ON DELETE RESTRICT,
		relative_path TEXT NOT NULL UNIQUE,
		accepted_at TEXT NOT NULL,
		PRIMARY KEY(student_id,requirement_id)
	)`
	if _, err := s.DB.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("initialize accepted files index: %w", err)
	}
	if err := s.initFeedback(ctx); err != nil { return err }
	return s.initJournal(ctx)
}

// safeName removes characters forbidden by major desktop filesystems while
// preserving readable Cyrillic text and whitespace.
// RU: safeName сохраняет читаемую кириллицу, удаляя опасные символы имени.
func safeName(value string) string {
	var b strings.Builder
	for _, char := range strings.TrimSpace(value) {
		switch char {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			b.WriteRune('_')
		default:
			if unicode.IsControl(char) {
				b.WriteRune('_')
			} else {
				b.WriteRune(char)
			}
		}
	}
	name := strings.Trim(b.String(), ". ")
	if name == "" {
		return "Без названия"
	}
	runes := []rune(name)
	if len(runes) > 80 {
		name = string(runes[:80])
	}
	// Windows device names cannot be used as filenames without a prefix.
	// RU: Избегаем зарезервированных имён Windows.
	upper := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	switch upper {
	case "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		name = "_" + name
	}
	return name
}

type versionInfo struct {
	studentID, requirementID, groupID                    int64
	name, educationalID, groupName, title, relative, sha string
}

// versionInfo verifies the scope and retrieves human-readable names before
// opening the SQL transaction for file projection.
// RU: versionInfo проверяет владельца версии и получает читаемые названия.
func (s *Service) versionInfo(ctx context.Context, studentID, requirementID, versionID int64) (versionInfo, error) {
	if studentID <= 0 || requirementID <= 0 || versionID <= 0 {
		return versionInfo{}, ErrInvalidVersion
	}
	const query = `SELECT s.id,r.id,g.id,s.full_name,s.educational_id,g.name,r.title,fv.relative_path,fv.sha256
		FROM file_versions fv JOIN students s ON fv.student_id=s.id
		JOIN requirements r ON fv.requirement_id=r.id
		JOIN student_groups g ON s.group_id=g.id AND r.group_id=g.id
		WHERE fv.id=? AND fv.student_id=? AND fv.requirement_id=?
		AND (r.student_id IS NULL OR r.student_id=s.id)`
	var result versionInfo
	err := s.DB.QueryRowContext(ctx, query, versionID, studentID, requirementID).
		Scan(&result.studentID, &result.requirementID, &result.groupID,
			&result.name, &result.educationalID, &result.groupName, &result.title, &result.relative, &result.sha)
	if errors.Is(err, sql.ErrNoRows) {
		return versionInfo{}, ErrInvalidVersion
	}
	if err != nil {
		return versionInfo{}, fmt.Errorf("look up document version: %w", err)
	}
	return result, nil
}

// projectionPath produces the readable group/student/document hierarchy.
// The internal student ID guarantees folder uniqueness even after sanitizing.
// RU: projectionPath формирует понятные папки с уникальным ID студента.
func projectionPath(info versionInfo, versionExt string) string {
	groupFolder := fmt.Sprintf("%s [%d]", safeName(info.groupName), info.groupID)
	studentFolder := fmt.Sprintf("%s [%s-%d]", safeName(info.name), safeName(info.educationalID), info.studentID)
	return path.Join(groupFolder, studentFolder, safeName(info.title)+versionExt)
}

// findProjection resolves collisions with previously approved documents.
// RU: findProjection исключает случайное совпадение имён двух требований.
func (s *Service) findProjection(ctx context.Context, wanted string, studentID, requirementID int64) (string, error) {
	var count int
	err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM accepted_files
		WHERE relative_path=? AND NOT(student_id=? AND requirement_id=?)`, wanted, studentID, requirementID).Scan(&count)
	if err != nil {
		return "", fmt.Errorf("check accepted document path: %w", err)
	}
	if count == 0 {
		return wanted, nil
	}
	ext := filepath.Ext(wanted)
	return strings.TrimSuffix(wanted, ext) + " [R" + strconv.FormatInt(requirementID, 10) + "]" + ext, nil
}

// Accept approves one immutable version and projects its bytes into a clean
// group/student/document path. The previous accepted copy stays intact until
// the new bytes are ready; in case of a database failure, the old copy is
// restored. The complete history remains in .internal/versions.
//
// RU: Accept принимает выбранную версию и копирует её в понятную папку.
// Старая принятая копия защищена до подготовки новой; при ошибке SQLite
// выполняется возврат старого файла. История версий остаётся нетронутой.
func (s *Service) Accept(ctx context.Context, studentID, requirementID, versionID int64) (Accepted, error) {
	approvalMu.Lock()
	defer approvalMu.Unlock()
	if err := ctx.Err(); err != nil {return Accepted{},err}
	info,err:=s.versionInfo(ctx,studentID,requirementID,versionID)
	if err!=nil{return Accepted{},err}
	data,err:=s.Versions.Read(ctx,versionID)
	if err!=nil{return Accepted{},fmt.Errorf("read approved source: %w",err)}
	if fingerprint(data)!=info.sha {
		return Accepted{},errors.New("document version checksum mismatch")
	}
	relative,err:=s.findProjection(ctx,projectionPath(info,filepath.Ext(info.relative)),studentID,requirementID)
	if err!=nil{return Accepted{},err}
	relative,err=storagepath.Normalize(relative)
	if err!=nil{return Accepted{},fmt.Errorf("invalid projected path: %w",err)}
	var previousPath string
	var previousVersion int64
	err=s.DB.QueryRowContext(ctx,`SELECT relative_path,version_id FROM accepted_files
	WHERE student_id=? AND requirement_id=?`,studentID,requirementID).
	Scan(&previousPath,&previousVersion)
	if err!=nil&&!errors.Is(err,sql.ErrNoRows){return Accepted{},fmt.Errorf("find previous approval: %w",err)}
	if errors.Is(err,sql.ErrNoRows){previousPath="";previousVersion=0}
	if previousPath!=""{
		previousPath,err=storagepath.Normalize(previousPath)
		if err!=nil{return Accepted{},fmt.Errorf("invalid prior approval path: %w",err)}
	}
	var pending int
	if err:=s.DB.QueryRowContext(ctx,`SELECT COUNT(*) FROM approval_operations
	WHERE student_id=? AND requirement_id=?`,studentID,requirementID).Scan(&pending);err!=nil{
		return Accepted{},err
	}
	if pending!=0{return Accepted{},ErrPendingApproval}

	// Verify the current public file against its immutable recorded version
	// before any replacement. Do not silently destroy user-edited content.
	// RU: Не заменяем правленый вручную принятый документ без предупреждения.
	previousHash:=""
	if previousVersion>0{
		oldBytes,err:=s.Versions.Read(ctx,previousVersion)
		if err!=nil{return Accepted{},fmt.Errorf("prior accepted source is invalid: %w",err)}
		previousHash=fingerprint(oldBytes)
		oldTarget,err:=secureTarget(s.Versions.Root,previousPath,false)
		if err!=nil{return Accepted{},fmt.Errorf("prior approved file is unavailable: %w",err)}
		foundHash,present,err:=existingHash(oldTarget)
		if err!=nil{return Accepted{},err}
		if !present||foundHash!=previousHash{return Accepted{},ErrRecoveryConflict}
	}
	if previousVersion==versionID&&previousPath==relative{
		return Accepted{StudentID:studentID,RequirementID:requirementID,
			VersionID:versionID,RelativePath:relative},nil
	}
	target,err:=secureTarget(s.Versions.Root,relative,true)
	if err!=nil{return Accepted{},fmt.Errorf("unsafe approved target: %w",err)}
	if previousPath!=relative {
		if _,present,err:=existingHash(target);err!=nil{return Accepted{},err
		}else if present{return Accepted{},ErrRecoveryConflict}
	}
	random:=make([]byte,12)
	if _,err:=rand.Read(random);err!=nil{return Accepted{},err}
	token:=hex.EncodeToString(random)
	temporary:=target+".tmp-"+token
	file,err:=os.OpenFile(temporary,os.O_WRONLY|os.O_CREATE|os.O_EXCL,0600)
	if err!=nil{return Accepted{},fmt.Errorf("create temporary approval: %w",err)}
	defer os.Remove(temporary)
	if _,err=file.Write(data);err!=nil{file.Close();return Accepted{},err}
	if err=file.Sync();err!=nil{file.Close();return Accepted{},err}
	if err=file.Close();err!=nil{return Accepted{},err}

	op:=approvalIntent{Token:token,StudentID:studentID,RequirementID:requirementID,
		VersionID:versionID,PreviousVersionID:previousVersion,
		PreviousPath:previousPath,TargetPath:relative,
		CreatedAt:approvalTimestamp(s.Clock)}
	// The durable intent is committed independently BEFORE any file rename.
	// RU: При сбое после этого этапа новая попытка запуска знает, что делать.
	if err:=s.insertIntent(ctx,op);err!=nil{return Accepted{},err}
	interrupt:=func(stage string) error {
		if s.interruptApproval!=nil{return s.interruptApproval(stage)}
		return nil
	}
	if err:=interrupt("after_intent");err!=nil{return Accepted{},err}
	backup:=target+".backup-"+token
	if previousPath==relative&&previousVersion>0{
		if err:=os.Rename(target,backup);err!=nil{
			return Accepted{},fmt.Errorf("%w: protect previous approved bytes: %v",ErrPendingApproval,err)
		}
		if err:=interrupt("after_old_renamed");err!=nil{return Accepted{},err}
	}
	if err:=os.Rename(temporary,target);err!=nil{
		return Accepted{},fmt.Errorf("%w: publish approved bytes: %v",ErrPendingApproval,err)
	}
	if err:=interrupt("after_publish");err!=nil{return Accepted{},err}
	tx,err:=s.DB.BeginTx(ctx,nil)
	if err!=nil{return Accepted{},fmt.Errorf("%w: begin SQLite commit: %v",ErrPendingApproval,err)}
	defer tx.Rollback()
	now:=s.Clock().UTC().Format(time.RFC3339Nano)
	_,err=tx.ExecContext(ctx,`INSERT INTO accepted_files(
	student_id,requirement_id,version_id,relative_path,accepted_at)
	VALUES(?,?,?,?,?) ON CONFLICT(student_id,requirement_id) DO UPDATE SET
	version_id=excluded.version_id,relative_path=excluded.relative_path,accepted_at=excluded.accepted_at`,
	studentID,requirementID,versionID,relative,now)
	if err!=nil{return Accepted{},fmt.Errorf("%w: save approved metadata: %v",ErrPendingApproval,err)}
	if _,err:=tx.ExecContext(ctx,"UPDATE file_versions SET accepted=1 WHERE id=?",versionID);err!=nil{
		return Accepted{},fmt.Errorf("%w: update approved revision: %v",ErrPendingApproval,err)
	}
	if _,err:=tx.ExecContext(ctx,`INSERT INTO material_status(
	student_id,requirement_id,status,changed_at) VALUES(?,?,?,?)
	ON CONFLICT(student_id,requirement_id) DO UPDATE SET
	status=excluded.status,changed_at=excluded.changed_at`,
	studentID,requirementID,string(materials.Accepted),now);err!=nil{
		return Accepted{},fmt.Errorf("%w: update review status: %v",ErrPendingApproval,err)
	}
	if _,err:=tx.ExecContext(ctx,`INSERT INTO material_events(
	student_id,requirement_id,status,occurred_at) VALUES(?,?,?,?)`,
	studentID,requirementID,string(materials.Accepted),now);err!=nil{
		return Accepted{},fmt.Errorf("%w: record approval history: %v",ErrPendingApproval,err)
	}
	if err:=tx.Commit();err!=nil{
		return Accepted{},fmt.Errorf("%w: commit approval: %v",ErrPendingApproval,err)
	}
	if err:=interrupt("after_commit");err!=nil{return Accepted{},err}
	// Use the same idempotent cleanup as next-start recovery. This may
	// remove ONLY the known old backup and previously accepted projection.
	// RU: После commit удаляем только проверенную старую копию.
	if err:=s.resolveIntent(ctx,op);err!=nil{
		return Accepted{},fmt.Errorf("%w: finalize accepted projection: %v",ErrPendingApproval,err)
	}
	return Accepted{StudentID:studentID,RequirementID:requirementID,
		VersionID:versionID,RelativePath:relative},nil
}

// Reject marks a submitted version for revision. Old accepted files and all
// internal version contents are intentionally preserved.
//
// RU: Reject отправляет документ на доработку, не удаляя принятые версии.
func (s *Service) Reject(ctx context.Context, studentID, requirementID, versionID int64) error {
	if _, err := s.versionInfo(ctx, studentID, requirementID, versionID); err != nil {
		return err
	}
	_, err := materials.NewRepository(s.DB).Set(ctx, studentID, requirementID, materials.NeedsRevision)
	return err
}
