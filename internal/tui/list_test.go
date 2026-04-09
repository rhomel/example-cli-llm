package tui

import (
	"strings"
	"testing"
)

func TestListModelUpdateMovesSelection(t *testing.T) {
	t.Parallel()

	model := NewListModel([]string{"one", "two", "three"})
	if action := model.Update(KeyDown); action != ActionContinue {
		t.Fatalf("action = %v", action)
	}
	if got := model.Selected(); got != "two" {
		t.Fatalf("Selected() = %q", got)
	}
	if action := model.Update(KeyUp); action != ActionContinue {
		t.Fatalf("action = %v", action)
	}
	if got := model.Selected(); got != "one" {
		t.Fatalf("Selected() = %q", got)
	}
}

func TestListModelAcceptAndCancel(t *testing.T) {
	t.Parallel()

	model := NewListModel([]string{"one"})
	if action := model.Update(KeyEnter); action != ActionAccept {
		t.Fatalf("action = %v, want accept", action)
	}
	if action := model.Update(KeyCtrlC); action != ActionCancel {
		t.Fatalf("action = %v, want cancel", action)
	}
}

func TestListModelViewRendersSingleSelection(t *testing.T) {
	t.Parallel()

	model := NewListModel([]string{"one", "two"})
	model.Update(KeyDown)

	view := model.View()
	if strings.Contains(view, "example\n") {
		t.Fatalf("view = %q", view)
	}
	if !strings.Contains(view, "> \x1b[7mtwo\x1b[0m") {
		t.Fatalf("view = %q", view)
	}
	if strings.Count(view, "\x1b[7m") != 1 {
		t.Fatalf("view = %q", view)
	}
}

func TestPadViewToBottom(t *testing.T) {
	t.Parallel()

	view := "a\nb\n"
	got := padViewToBottom(view, 5)

	if got != "\n\n\na\nb\n" {
		t.Fatalf("padViewToBottom() = %q", got)
	}
}
