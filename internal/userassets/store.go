// Package userassets stores immutable teacher-supplied document templates
// separately from student submissions, inside the portable backup tree.
// Пакет userassets хранит версии пользовательских шаблонов в storage.
package userassets

import (
	"bytes"
	"archive/zip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/ZakirAlekperov/ThesisFlow/internal/storagepath"
)

const MaxBytes = 8 << 20

var ErrInvalid = errors.New("invalid document template")

type Asset struct {
	ID                                  int64
	Name, Description, RequirementTitle string
	CurrentVersion                      int
}
type Version struct {
	ID, AssetID                    int64
	Number                         int
	Filename, RelativePath, SHA256 string
	Size                           int64
}
type Store struct {
	DB   *sql.DB
	Root string
}

func New(db *sql.DB, root string) *Store { return &Store{DB: db, Root: root} }
func validText(value string, max int) bool {
	return strings.TrimSpace(value) != "" && len([]rune(value)) <= max
}
func fileExtension(filename string) string { return strings.ToLower(filepath.Ext(filename)) }
func validateContent(filename string, data []byte) bool {
	if len(data) == 0 || len(data) > MaxBytes {
		return false
	}
	switch fileExtension(filename) {
	case ".txt":
		return utf8.Valid(data) && bytes.IndexByte(data, 0) < 0
	case ".pdf":
		return bytes.HasPrefix(data, []byte("%PDF-"))
	case ".docx", ".xlsx":
		// Check the ZIP central directory and expected OOXML parts without
		// extracting arbitrary paths or running macros.
		// RU: Переименованный не-ZIP не является шаблоном DOCX/XLSX.
		if !bytes.HasPrefix(data,[]byte("PK\x03\x04")){return false}
		archive,err:=zip.NewReader(bytes.NewReader(data),int64(len(data)))
		if err!=nil||len(archive.File)>3000{return false}
		contentType:=false; mainPart:=false
		expected:="word/document.xml"
		if fileExtension(filename)==".xlsx"{expected="xl/workbook.xml"}
		for _,entry:=range archive.File{
			if entry.Name=="[Content_Types].xml"{contentType=true}
			if entry.Name==expected{mainPart=true}
		}
		return contentType&&mainPart
	default:
		return false
	}
}

// Create stores metadata, not a file, with no relationship to students.
// RU: Отдельный логический шаблон создаётся без студенческих файлов.
func (s *Store) Create(ctx context.Context, name, description, requirementTitle string) (Asset, error) {
	if !validText(name, 120) || len([]rune(description)) > 800 || len([]rune(requirementTitle)) > 120 {
		return Asset{}, ErrInvalid
	}
	result, err := s.DB.ExecContext(ctx, `INSERT INTO template_assets(name,description,requirement_title)
 VALUES(?,?,?)`, strings.TrimSpace(name), description, requirementTitle)
	if err != nil {
		return Asset{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Asset{}, err
	}
	return Asset{ID: id, Name: strings.TrimSpace(name), Description: description, RequirementTitle: requirementTitle}, nil
}
func (s *Store) List(ctx context.Context) ([]Asset, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT id,name,description,requirement_title,current_version FROM template_assets ORDER BY name,id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Asset{}
	for rows.Next() {
		var a Asset
		if err := rows.Scan(&a.ID, &a.Name, &a.Description, &a.RequirementTitle, &a.CurrentVersion); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// AddVersion preserves prior bytes; file publication happens before SQL
// commit, so crash leftovers are inventory warnings rather than lost links.
// RU: Замена создаёт новую неизменяемую версию, старые файлы остаются.
func (s *Store) AddVersion(ctx context.Context, assetID int64, name string, data []byte) (Version, error) {
	if assetID <= 0 || len([]rune(name)) > 180 || strings.ContainsAny(name, "/\\:") || !validateContent(name, data) {
		return Version{}, ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Version{}, err
	}
	defer tx.Rollback()
	var number int
	if err := tx.QueryRowContext(ctx, "SELECT current_version FROM template_assets WHERE id=?", assetID).Scan(&number); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Version{}, ErrInvalid
		}
		return Version{}, err
	}
	sum := sha256.Sum256(data)
	v := Version{AssetID: assetID, Number: number + 1, Filename: name, Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:])}
	// A unique random file name is unnecessary: the DB-issued version ID is
	// stable and the relative path has no user-controlled components.
	res, err := tx.ExecContext(ctx, `INSERT INTO template_versions(asset_id,number,filename,relative_path,size,sha256)
 VALUES(?,?,?,'pending',?,?)`, assetID, v.Number, name, v.Size, v.SHA256)
	if err != nil {
		return Version{}, err
	}
	v.ID, err = res.LastInsertId()
	if err != nil {
		return Version{}, err
	}
	v.RelativePath = path.Join(".internal", "templates", fmt.Sprint(assetID), fmt.Sprintf("%d%s", v.ID, fileExtension(name)))
	target := filepath.Join(s.Root, filepath.FromSlash(v.RelativePath))
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return Version{}, err
	}
	temp, err := os.CreateTemp(filepath.Dir(target), ".template-*")
	if err != nil {
		return Version{}, err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return Version{}, err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return Version{}, err
	}
	if err := temp.Close(); err != nil {
		return Version{}, err
	}
	if err := os.Rename(tempName, target); err != nil {
		return Version{}, err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE template_versions SET relative_path=? WHERE id=?", v.RelativePath, v.ID); err != nil {
		return Version{}, err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE template_assets SET current_version=? WHERE id=?", v.Number, assetID); err != nil {
		return Version{}, err
	}
	if err := tx.Commit(); err != nil {
		return Version{}, err
	}
	return v, nil
}
func (s *Store) Versions(ctx context.Context, assetID int64) ([]Version, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id,asset_id,number,filename,relative_path,size,sha256 FROM template_versions
 WHERE asset_id=? ORDER BY number DESC`, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Version{}
	for rows.Next() {
		var v Version
		if err := rows.Scan(&v.ID, &v.AssetID, &v.Number, &v.Filename, &v.RelativePath, &v.Size, &v.SHA256); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}

// Read revalidates path, size and SHA-256; it never trusts incoming URL paths.
// RU: Скачать можно только зарегистрированную и проверенную версию.
func (s *Store) Read(ctx context.Context, assetID, versionID int64) (Version, []byte, error) {
	var v Version
	err := s.DB.QueryRowContext(ctx, `SELECT id,asset_id,number,filename,relative_path,size,sha256
 FROM template_versions WHERE id=? AND asset_id=?`, versionID, assetID).
		Scan(&v.ID, &v.AssetID, &v.Number, &v.Filename, &v.RelativePath, &v.Size, &v.SHA256)
	if err != nil {
		return Version{}, nil, ErrInvalid
	}
	clean, err := storagepath.Native(v.RelativePath)
	if err != nil || !strings.HasPrefix(filepath.ToSlash(clean), ".internal/templates/") {
		return Version{}, nil, ErrInvalid
	}
	file := filepath.Join(s.Root, clean)
	for p := filepath.Dir(file); p != s.Root && p != filepath.Dir(p); p = filepath.Dir(p) {
		stat, err := os.Lstat(p)
		if err != nil || stat.Mode()&os.ModeSymlink != 0 {
			return Version{}, nil, ErrInvalid
		}
	}
	stat, err := os.Lstat(file)
	if err != nil || !stat.Mode().IsRegular() || stat.Size() != v.Size || v.Size > MaxBytes {
		return Version{}, nil, ErrInvalid
	}
	fd, err := os.Open(file)
	if err != nil {
		return Version{}, nil, err
	}
	defer fd.Close()
	data, err := io.ReadAll(io.LimitReader(fd, MaxBytes+1))
	if err != nil {
		return Version{}, nil, err
	}
	sum := sha256.Sum256(data)
	if int64(len(data)) != v.Size || !strings.EqualFold(hex.EncodeToString(sum[:]), v.SHA256) {
		return Version{}, nil, ErrInvalid
	}
	return v, data, nil
}

// RequirementBinding exposes the optional currently selected teacher file
// template for a GROUP requirement. Submission does not depend on it.
// RU: Привязка шаблона не влияет на сдачу или принятие документа.
type RequirementBinding struct{RequirementID int64;Title string;AssetID int64}

// ListBindings returns current mandatory group requirements, including
// ones without a linked template.
// RU: Отсутствие шаблона — допустимое состояние.
func(s *Store)ListBindings(ctx context.Context,groupID int64)([]RequirementBinding,error){
 rows,err:=s.DB.QueryContext(ctx,`SELECT r.id,r.title,COALESCE(b.asset_id,0)
 FROM requirements r LEFT JOIN requirement_asset_bindings b ON b.requirement_id=r.id
 WHERE r.group_id=? AND r.student_id IS NULL ORDER BY r.title_key,r.id`,groupID)
 if err!=nil{return nil,err};defer rows.Close()
 list:=[]RequirementBinding{}
 for rows.Next(){var x RequirementBinding
  if err:=rows.Scan(&x.RequirementID,&x.Title,&x.AssetID);err!=nil{return nil,err}
  list=append(list,x)
 }
 return list,rows.Err()
}
// Bind attaches an existing teacher asset to a group requirement or clears
// the association with assetID==0. It never changes document status.
// RU: Назначение файла-шаблона не меняет учебные документы и события.
func(s *Store)Bind(ctx context.Context,groupID,requirementID,assetID int64)error{
 if groupID<=0||requirementID<=0||assetID<0{return ErrInvalid}
 tx,err:=s.DB.BeginTx(ctx,nil);if err!=nil{return err};defer tx.Rollback()
 var exists int
 if err:=tx.QueryRowContext(ctx,`SELECT EXISTS(SELECT 1 FROM requirements
 WHERE id=? AND group_id=? AND student_id IS NULL)`,requirementID,groupID).Scan(&exists);err!=nil{return err}
 if exists!=1{return ErrInvalid}
 if assetID!=0{
  if err:=tx.QueryRowContext(ctx,"SELECT EXISTS(SELECT 1 FROM template_assets WHERE id=?)",assetID).Scan(&exists);err!=nil{return err}
  if exists!=1{return ErrInvalid}
 }
 if assetID==0{
  _,err=tx.ExecContext(ctx,"DELETE FROM requirement_asset_bindings WHERE requirement_id=?",requirementID)
 }else{
  _,err=tx.ExecContext(ctx,`INSERT INTO requirement_asset_bindings(requirement_id,asset_id) VALUES(?,?)
  ON CONFLICT(requirement_id) DO UPDATE SET asset_id=excluded.asset_id`,requirementID,assetID)
 }
 if err!=nil{return err}
 return tx.Commit()
}
