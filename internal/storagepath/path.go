// Package storagepath normalizes safe relative file names across Windows,
// macOS and Linux without changing the stored user's data.
// Пакет storagepath безопасно обрабатывает переносимые относительные пути.
package storagepath

import (
 "errors"
 "path"
 "path/filepath"
 "strings"
)

// ErrUnsafe is returned for traversal, absolute, UNC, drive or malformed paths.
// RU: ErrUnsafe запрещает выход за каталог, абсолютные и опасные пути.
var ErrUnsafe=errors.New("unsafe relative storage path")

// Normalize uses '/' separators in SQLite. It also accepts safe legacy
// Windows-relative paths containing backslashes, without interpreting drive
// letters or UNC shares as legitimate workspace-relative locations.
// RU: Старые Windows-пути читаются, новые хранятся с '/'.
func Normalize(raw string)(string,error){
 if raw==""||strings.ContainsAny(raw,":\x00"){return "",ErrUnsafe}
 clean:=strings.ReplaceAll(raw,"\\","/")
 if clean=="."||!path.IsLocal(clean)||path.Clean(clean)!=clean{return "",ErrUnsafe}
 for _,part:=range strings.Split(clean,"/"){
  if part==""||part=="."||part==".."{return "",ErrUnsafe}
 }
 return clean,nil
}

// Native converts a verified portable relative path to a native OS path.
// RU: Обратно-совместимое преобразование безопасного пути для os.Open.
func Native(raw string)(string,error){
 normal,err:=Normalize(raw)
 if err!=nil{return "",err}
 return filepath.FromSlash(normal),nil
}
