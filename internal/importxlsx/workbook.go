package importxlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"path"
	"strings"
)

// ErrSheetNotFound identifies a requested tab absent from a workbook.
// RU: ErrSheetNotFound означает, что вкладка XLSX не найдена.
var ErrSheetNotFound = errors.New("XLSX worksheet not found")

type workbookDirectory struct {
	Sheets []struct {
		Name string `xml:"name,attr"`
		RelID string `xml:"id,attr"`
	} `xml:"sheets>sheet"`
}

type workbookRelations struct {
	Items []struct {
		ID string `xml:"Id,attr"`
		Type string `xml:"Type,attr"`
		Target string `xml:"Target,attr"`
		TargetMode string `xml:"TargetMode,attr"`
	} `xml:"Relationship"`
}

// workbookSheets discovers actual worksheet paths using Office Open XML
// workbook relationship IDs, rather than guessing sheet1.xml.
// RU: workbookSheets читает имена листов и реальные пути через связи XLSX.
func workbookSheets(data []byte) (map[string]string, []string, error) {
	if len(data) > MaxFileSize {return nil,nil,ErrTooLarge}
	archive,err:=zip.NewReader(bytes.NewReader(data),int64(len(data)))
	if err!=nil{return nil,nil,fmt.Errorf("open XLSX: %w",err)}
	var bookXML,relsXML []byte
	for _,entry:=range archive.File {
		if entry.Name!="xl/workbook.xml" && entry.Name!="xl/_rels/workbook.xml.rels" {continue}
		content,err:=readPart(entry)
		if err!=nil{return nil,nil,err}
		if entry.Name=="xl/workbook.xml"{bookXML=content}else{relsXML=content}
	}
	if len(bookXML)==0 || len(relsXML)==0{return nil,nil,errors.New("XLSX workbook metadata missing")}
	var book workbookDirectory
	if err:=xml.Unmarshal(bookXML,&book);err!=nil{return nil,nil,fmt.Errorf("parse workbook: %w",err)}
	var relations workbookRelations
	if err:=xml.Unmarshal(relsXML,&relations);err!=nil{return nil,nil,fmt.Errorf("parse workbook relationships: %w",err)}
	ids:=make(map[string]string)
	for _,rel:=range relations.Items {
		if !strings.HasSuffix(rel.Type,"/worksheet") || rel.TargetMode!="" {continue}
		if strings.Contains(rel.Target,"..") || strings.Contains(rel.Target,"\\") {continue}
		var resolved string
		if strings.HasPrefix(rel.Target,"/") {
			resolved=path.Clean(strings.TrimPrefix(rel.Target,"/"))
		} else {
			resolved=path.Clean(path.Join("xl",rel.Target))
		}
		if !strings.HasPrefix(resolved,"xl/worksheets/") || !strings.HasSuffix(resolved,".xml"){continue}
		ids[rel.ID]=resolved
	}
	names:=make([]string,0,len(book.Sheets))
	paths:=make(map[string]string)
	if len(book.Sheets)>256 {return nil,nil,ErrTooLarge}
	for _,entry:=range book.Sheets {
		if entry.Name=="" {return nil,nil,errors.New("XLSX sheet has no name")}
		resolved,ok:=ids[entry.RelID]
		if !ok{return nil,nil,fmt.Errorf("XLSX worksheet relationship missing: %s",entry.Name)}
		if _,duplicate:=paths[entry.Name];duplicate{return nil,nil,errors.New("duplicate sheet name")}
		names=append(names,entry.Name)
		paths[entry.Name]=resolved
	}
	return paths,names,nil
}

// Sheets returns the visible tab names in Excel workbook order.
// RU: Sheets возвращает названия вкладок в порядке книги Excel.
func Sheets(data []byte) ([]string,error) {
	_,names,err:=workbookSheets(data)
	return names,err
}

// ReadNamed reads one selected worksheet without importing any database rows.
// RU: ReadNamed читает выбранную вкладку без изменения SQLite.
func ReadNamed(data []byte,name string) ([][]string,error) {
	paths,names,err:=workbookSheets(data)
	if err!=nil{return nil,err}
	if name=="" && len(names)>0{name=names[0]}
	selected,ok:=paths[name]
	if !ok{return nil,ErrSheetNotFound}
	return readSelected(data,selected)
}
