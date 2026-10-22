package web

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/analytics"
	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// TestAnalyticsBrowserAndExportsUseSameCurrentSQLiteState ensures the
// displayed filters correspond exactly to downloadable CSV/XLSX documents.
// RU: Экспорт формируется из того же актуального состояния, что экран.
func TestAnalyticsBrowserAndExportsUseSameCurrentSQLiteState(t *testing.T) {
	h, groupID, studentID, ruleID, _, versions := documentFixture(t)
	ctx := context.Background()
	_ = studentID
	provider := analytics.New(versions.DB)
	reporter, err := NewHandlerWithServices(groups.NewRepository(versions.DB),
		students.NewRepository(versions.DB), Dependencies{Analytics: provider})
	if err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/reports?group=%d&status=missing", groupID)
	rec := httptest.NewRecorder()
	reporter.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Глава 1") ||
		!strings.Contains(rec.Body.String(), "Иванов Иван") || !strings.Contains(rec.Body.String(), "Аналитика и отчёты ВКР") {
		t.Fatalf("report HTML %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("analytics was cached")
	}
	rec = httptest.NewRecorder()
	reporter.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/reports/export.csv?group=%d&status=missing", groupID), nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Disposition"), ".csv") {
		t.Fatalf("CSV HTTP %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.HasPrefix(rec.Body.Bytes(), []byte{0xef, 0xbb, 0xbf}) {
		t.Error("CSV BOM missing")
	}
	reader := csv.NewReader(bytes.NewReader(rec.Body.Bytes()[3:]))
	records, err := reader.ReadAll()
	if err != nil || len(records) != 2 {
		t.Fatalf("CSV data %+v error %v", records, err)
	}
	if records[1][3] != "Глава 1" || records[1][4] != "Не сдан" {
		t.Errorf("CSV mismatch %v", records[1])
	}
	rec = httptest.NewRecorder()
	reporter.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/reports/export.xlsx?group=%d", groupID), nil))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Fatalf("xlsx HTTP %d content %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	archive, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil || len(archive.File) < 8 {
		t.Fatalf("invalid OOXML zip: %v %v", err, archive)
	}
	found := false
	for _, part := range archive.File {
		if part.Name == "xl/worksheets/sheet3.xml" {
			file, err := part.Open()
			if err != nil {
				t.Fatal(err)
			}
			raw, err := io.ReadAll(file)
			_ = file.Close()
			if err != nil {
				t.Fatal(err)
			}
			found = strings.Contains(string(raw), "Глава 1")
		}
	}
	if !found {
		t.Error("document table was not exported")
	}
	// A status change via real document upload appears in the next GET.
	page := documentURL(groupID, studentID)
	token := documentToken(t, h, page)
	uploadReviewedVersion(t, h, page, token, ruleID)
	rec = httptest.NewRecorder()
	reporter.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/reports?status=under_review", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "На проверке") {
		t.Fatalf("uploaded version absent from report %d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	reporter.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/reports/print", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Security-Policy"), "default-src") {
		t.Errorf("print output %d CSP %q", rec.Code, rec.Header().Get("Content-Security-Policy"))
	}
	_ = ctx
}

func TestAnalyticsReportInvalidFiltersAndGroupScope(t *testing.T) {
	_, groupID, _, _, _, versions := documentFixture(t)
	reporter, err := NewHandlerWithServices(groups.NewRepository(versions.DB),
		students.NewRepository(versions.DB), Dependencies{Analytics: analytics.New(versions.DB)})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path string
		code int
	}{
		{"/reports?sort=DROP", http.StatusBadRequest},
		{"/reports?status=invalid", http.StatusBadRequest},
		{"/reports?group=-1", http.StatusBadRequest},
		{"/reports?overdue=maybe", http.StatusBadRequest},
		{"/reports?from=2026-02-30", http.StatusBadRequest},
		{"/reports?from=2026-10-30&to=2026-10-01", http.StatusBadRequest},
		{"/reports/groups/987654321", http.StatusNotFound},
		{fmt.Sprintf("/reports/groups/%d", groupID), http.StatusOK},
	} {
		rec := httptest.NewRecorder()
		reporter.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if rec.Code != tc.code {
			t.Errorf("%s: HTTP %d want %d", tc.path, rec.Code, tc.code)
		}
	}
}
