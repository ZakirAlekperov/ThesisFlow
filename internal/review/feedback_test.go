package review

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/materials"
)

// TestRejectWithReasonPersistsDecision checks that a rejection writes the
// status transition and an immutable, version-specific reason together.
// RU: Причина возврата и новый статус сохраняются одной транзакцией.
func TestRejectWithReasonPersistsDecision(t *testing.T) {
	s, studentID, requirementID := testReview(t)
	ctx := context.Background()
	version, err := s.Versions.Save(ctx, studentID, requirementID, "draft.docx", []byte("first version"))
	if err != nil { t.Fatal(err) }
	if err := s.RejectWithReason(ctx, studentID, requirementID, version.ID, "Не хватает обоснования выбора метода."); err != nil {
		t.Fatal(err)
	}
	status, err := materials.NewRepository(s.DB).Get(ctx, studentID, requirementID)
	if err != nil || status.Status != materials.NeedsRevision {
		t.Fatalf("status after rejection = %+v (%v)", status, err)
	}
	notes, err := s.ListFeedback(ctx, studentID, requirementID)
	if err != nil || len(notes) != 1 { t.Fatalf("feedback = %+v, %v", notes, err) }
	if notes[0].VersionID != version.ID || notes[0].Kind != FeedbackRevision || notes[0].Text != "Не хватает обоснования выбора метода." || notes[0].CreatedAt.IsZero() {
		t.Fatalf("persisted feedback: %+v", notes[0])
	}
	// A second upload keeps earlier review history unchanged.
	_, err = s.Versions.Save(ctx, studentID, requirementID, "revised.docx", []byte("second version"))
	if err != nil { t.Fatal(err) }
	notes, err = s.ListFeedback(ctx, studentID, requirementID)
	if err != nil || len(notes) != 1 || notes[0].VersionID != version.ID { t.Fatalf("history after new upload: %+v %v", notes, err) }
}

// TestRejectionAndReasonAreAtomic forces SQLite to fail feedback insertion.
// Neither material state nor its history may be modified.
// RU: Ошибка записи замечания должна откатить изменение статуса.
func TestRejectionAndReasonAreAtomic(t *testing.T) {
	s, studentID, requirementID := testReview(t)
	ctx := context.Background()
	version, err := s.Versions.Save(ctx, studentID, requirementID, "draft.docx", []byte("draft"))
	if err != nil { t.Fatal(err) }
	before, err := materials.NewRepository(s.DB).History(ctx, studentID, requirementID)
	if err != nil { t.Fatal(err) }
	_, err = s.DB.ExecContext(ctx, "CREATE TRIGGER reject_feedback BEFORE INSERT ON review_feedback BEGIN SELECT RAISE(ABORT, 'test rollback'); END")
	if err != nil { t.Fatal(err) }
	if err := s.RejectWithReason(ctx, studentID, requirementID, version.ID, "Исправьте таблицу"); err == nil { t.Fatal("expected failure") }
	now, err := materials.NewRepository(s.DB).Get(ctx, studentID, requirementID)
	if err != nil || now.Status != materials.UnderReview { t.Fatalf("partial status update: %+v %v", now, err) }
	after, err := materials.NewRepository(s.DB).History(ctx, studentID, requirementID)
	if err != nil || len(after) != len(before) { t.Fatalf("partial history: %+v, %v", after, err) }
}

// TestReviewFeedbackScopeAndValidation prevents cross-student notes, empty
// reasons, and stale revision decisions.
// RU: Чужие файлы и старые версии нельзя отправлять на доработку.
func TestReviewFeedbackScopeAndValidation(t *testing.T) {
	s, studentID, requirementID := testReview(t)
	ctx := context.Background()
	first, err := s.Versions.Save(ctx, studentID, requirementID, "old.docx", []byte("old"))
	if err != nil { t.Fatal(err) }
	latest, err := s.Versions.Save(ctx, studentID, requirementID, "new.docx", []byte("new"))
	if err != nil { t.Fatal(err) }
	for _, tc := range []struct{ version int64; reason string; want error }{
		{latest.ID, " ", ErrInvalidFeedback},
		{latest.ID, strings.Repeat("я", MaxFeedbackRunes+1), ErrInvalidFeedback},
		{first.ID, "Старая версия", ErrStaleReview},
		{999999, "Чужая версия", ErrInvalidVersion},
	} {
		err := s.RejectWithReason(ctx, studentID, requirementID, tc.version, tc.reason)
		if !errors.Is(err, tc.want) { t.Errorf("reject(%d,%q) = %v, want %v", tc.version, tc.reason[:min(len(tc.reason),20)], err, tc.want) }
	}
	if _, err := s.AddComment(ctx, studentID+999, requirementID, latest.ID, "Чужая запись"); !errors.Is(err, ErrInvalidVersion) {
		t.Errorf("cross-scope comment: %v", err)
	}
	if _, err := s.AddComment(ctx, studentID, requirementID, latest.ID, "  "); !errors.Is(err, ErrInvalidFeedback) {
		t.Errorf("empty comment: %v", err)
	}
	notes, err := s.ListFeedback(ctx, studentID, requirementID)
	if err != nil || len(notes) != 0 { t.Fatalf("unexpected feedback: %+v %v", notes, err) }
}

// TestAddCommentDoesNotMutateMaterialStatus tests a note on a specific
// historical version without making a review decision.
// RU: Комментарий не должен менять состояние материала.
func TestAddCommentDoesNotMutateMaterialStatus(t *testing.T) {
	s, studentID, requirementID := testReview(t)
	ctx := context.Background()
	version, err := s.Versions.Save(ctx, studentID, requirementID, "draft.docx", []byte("content"))
	if err != nil { t.Fatal(err) }
	_, err = s.AddComment(ctx, studentID, requirementID, version.ID, "  Проверьте подпись к рисунку.  ")
	if err != nil { t.Fatal(err) }
	notes, err := s.ListFeedback(ctx, studentID, requirementID)
	if err != nil || len(notes) != 1 || notes[0].Text != "Проверьте подпись к рисунку." || notes[0].Kind != FeedbackComment {
		t.Fatalf("comment lost: %+v %v", notes, err)
	}
	current, err := materials.NewRepository(s.DB).Get(ctx, studentID, requirementID)
	if err != nil || current.Status != materials.UnderReview { t.Fatalf("comment changed status: %+v %v", current, err) }
}
