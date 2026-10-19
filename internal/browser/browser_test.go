package browser

import (
	"reflect"
	"testing"
)

// TestCommandForOS checks the browser-opening commands on supported systems.
// Scenario: resolve a fixed loopback URL for macOS, Windows, Linux and unknown OS.
// Expected: the chosen command passes the URL unmodified or returns an error.
//
// RU: Проверяет команды открытия браузера в разных ОС.
// Сценарий: один локальный URL для macOS, Windows, Linux и неизвестной ОС.
// Ожидается: исходный URL передаётся без изменений; неизвестная ОС отклоняется.
func TestCommandForOS(t *testing.T) {
	const address = "http://127.0.0.1:34567/"
	cases := []struct{
		name string
		goos string
		wantExe string
		wantArgs []string
		wantErr bool
	}{
		{"macOS", "darwin", "open", []string{address}, false},
		{"Windows", "windows", "rundll32", []string{"url.dll,FileProtocolHandler", address}, false},
		{"Linux", "linux", "xdg-open", []string{address}, false},
		{"unknown", "plan9", "", nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			exe, args, err := commandForOS(tc.goos, address)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr && (exe != tc.wantExe || !reflect.DeepEqual(args, tc.wantArgs)) {
				t.Errorf("command = (%q, %q), want (%q, %q)", exe, args, tc.wantExe, tc.wantArgs)
			}
		})
	}
}
