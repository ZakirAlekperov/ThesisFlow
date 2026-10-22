package storagepath

import (
 "path/filepath"
 "testing"
)

func TestNormalizeAcceptsPortableAndLegacyWindowsPaths(t *testing.T){
 for _,test:=range []struct{input,want string}{
  {".internal/versions/7/2/chapter.docx",".internal/versions/7/2/chapter.docx"},
  {".internal\\versions\\7\\2\\chapter.docx",".internal/versions/7/2/chapter.docx"},
  {"ИС-401\\Иванов Иван\\Глава 1.docx","ИС-401/Иванов Иван/Глава 1.docx"},
 }{
  got,err:=Normalize(test.input)
  if err!=nil||got!=test.want{t.Errorf("normalize %q = %q, %v",test.input,got,err)}
  native,err:=Native(test.input)
  if err!=nil||native!=filepath.FromSlash(test.want){t.Errorf("native %q = %q, %v",test.input,native,err)}
 }
}

// TestNormalizeRejectsTraversalAndAbsoluteNames prevents a legacy
// separator from turning an unsafe path into an allowed workspace path.
// RU: Независимо от ОС не разрешаем обход через '..' и UNC/диски.
func TestNormalizeRejectsTraversalAndAbsoluteNames(t *testing.T){
 for _,name:=range []string{
  "",".","..","../../file","a/../file","a\\..\\file",
  "a//file","a/./file","/etc/passwd","\\\\server\\share",
  "C:\\user\\file.docx","a:evil","a/","a\\", "\x00",
 }{
  if result,err:=Normalize(name);err==nil{t.Errorf("unsafe path accepted %q as %q",name,result)}
 }
}
