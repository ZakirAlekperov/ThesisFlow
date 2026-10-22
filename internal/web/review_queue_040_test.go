package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/requirements"
	"github.com/ZakirAlekperov/ThesisFlow/internal/reviewqueue"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// TestReviewQueueTracksLatestUploadedVersionThroughHTTP verifies that pending
// work is visible immediately and disappears after the latest review action.
// RU: Сквозная проверка очереди с реальными загрузками и приёмкой файлов.
func TestReviewQueueTracksLatestUploadedVersionThroughHTTP(t *testing.T) {
	h, groupID, studentID, reqID, _, versions := documentFixture(t)
	groupsRepo := groups.NewRepository(versions.DB)
	reader := reviewqueue.New(versions.DB)
	queue, err := NewHandlerWithServices(groupsRepo, students.NewRepository(versions.DB),
		Dependencies{ReviewQueue: reader})
	if err != nil {
		t.Fatal(err)
	}
	page := documentURL(groupID, studentID)
	token := documentToken(t, h, page)
	list := func(expected int) string {
		t.Helper()
		rec := httptest.NewRecorder()
		queue.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/reviews", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("queue http %d: %s", rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		if count := strings.Count(body, "Проверить</a>"); count != expected {
			t.Fatalf("pending items = %d expected %d: %s", count, expected, body)
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("queue may be cached")
		}
		return body
	}
	list(0)
	uploadReviewedVersion(t, h, page, token, reqID)
	list(1)
	uploadReviewedVersion(t, h, page, token, reqID)
	body := list(1)
	items, err := versions.List(context.Background(), studentID, reqID)
	if err != nil || len(items) != 2 {
		t.Fatalf("versions: %+v / %v", items, err)
	}
	if !strings.Contains(body, fmt.Sprintf("#version-%d", items[1].ID)) ||
		strings.Contains(body, fmt.Sprintf("#version-%d", items[0].ID)) {
		t.Fatalf("latest-only queue mismatch: %s", body)
	}
	form := url.Values{"csrf_token": {token}}
	req := httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/documents/%d/accept/%d", page, reqID, items[1].ID),
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("accept %d %s", rec.Code, rec.Body.String())
	}
	list(0)
	for _, path := range []string{"/reviews?offset=-1", "/reviews?group=-2", "/reviews?group=9999"} {
		rec = httptest.NewRecorder()
		queue.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
			t.Errorf("invalid query %s HTTP %d", path, rec.Code)
		}
	}
}

// TestCompletenessIncludesPersonalRequirementsAndNoFakeHundredPercent checks
// the definition of readiness against group-wide and personal requirements.
// RU: Персональные требования учитываются, нулевое количество не равно 100%.
func TestCompletenessIncludesPersonalRequirementsAndNoFakeHundredPercent(t *testing.T) {
	h, groupID, studentID, reqID, _, versions := documentFixture(t)
	ctx := context.Background()
	_ = h
	repo := requirements.NewRepository(versions.DB)
	personal, err := repo.Create(ctx, groupID, studentID, "Приложение индивидуальное", "2026-01-03")
	if err != nil {
		t.Fatal(err)
	}
	reader := reviewqueue.New(versions.DB)
	rows, err := reader.GroupCompleteness(ctx, groupID, time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC))
	if err != nil || len(rows) != 1 {
		t.Fatalf("completeness %+v err=%v", rows, err)
	}
	row := rows[0]
	if row.Total != 2 || row.Missing != 2 || row.Accepted != 0 || row.Overdue != 1 {
		t.Fatalf("group/personal overdue count wrong %+v", row)
	}
	// Reading must not create material_status records.
	var count int
	if err := versions.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM material_status").Scan(&count); err != nil || count != 0 {
		t.Fatalf("read-only completeness mutated DB: %d %v", count, err)
	}
	// A legitimate uploaded revision changes the current statuses.
	token := documentToken(t, h, documentURL(groupID, studentID))
	uploadReviewedVersion(t, h, documentURL(groupID, studentID), token, reqID)
	rows, err = reader.GroupCompleteness(ctx, groupID, time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC))
	if err != nil || rows[0].UnderReview != 1 || rows[0].Missing != 1 || rows[0].Overdue != 1 {
		t.Fatalf("wrong workflow completeness: %+v err=%v", rows, err)
	}
	// Personal requirement cannot disappear due to group-only matrix projection.
	if personal.StudentID != studentID {
		t.Fatal("personal ID changed")
	}
	g, err := groups.NewRepository(versions.DB).Create(ctx, "ПУСТАЯ")
	if err != nil {
		t.Fatal(err)
	}
	_, err = students.NewRepository(versions.DB).Create(ctx, g.ID, "ЗК-404", "Без требований")
	if err != nil {
		t.Fatal(err)
	}
	rows, err = reader.GroupCompleteness(ctx, g.ID, time.Now())
	if err != nil || len(rows) != 1 || rows[0].Total != 0 {
		t.Fatalf("group with no requirements: %+v %v", rows, err)
	}
	// Page renders an explicit undefined readiness instead of 100%.
	q, err := NewHandlerWithServices(groups.NewRepository(versions.DB),
		students.NewRepository(versions.DB), Dependencies{ReviewQueue: reader})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	q.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/groups/"+strconv.FormatInt(g.ID, 10)+"/completeness", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Нет требований") {
		t.Errorf("zero-requirement page: %d %s", rec.Code, rec.Body.String())
	}
}
