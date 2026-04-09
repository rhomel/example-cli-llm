package tui

import (
	"fmt"
	"strings"
)

type Action int

const (
	ActionContinue Action = iota
	ActionAccept
	ActionCancel
)

type ListModel struct {
	items    []string
	selected int
}

func NewListModel(items []string) ListModel {
	return ListModel{
		items: append([]string(nil), items...),
	}
}

func (m *ListModel) Update(key Key) Action {
	if len(m.items) == 0 {
		return ActionCancel
	}
	switch key {
	case KeyEnter:
		return ActionAccept
	case KeyCtrlC, KeyEsc:
		return ActionCancel
	case KeyUp, KeyK:
		m.selected = (m.selected - 1 + len(m.items)) % len(m.items)
	case KeyDown, KeyJ:
		m.selected = (m.selected + 1) % len(m.items)
	}
	return ActionContinue
}

func (m ListModel) Selected() string {
	if len(m.items) == 0 {
		return ""
	}
	return m.items[m.selected]
}

func (m ListModel) View() string {
	var b strings.Builder
	b.WriteString("example\n")
	b.WriteString("\n")
	for i, item := range m.items {
		cursor := " "
		line := item
		if i == m.selected {
			cursor = ">"
			line = inverse(line)
		}
		b.WriteString(fmt.Sprintf("%s %s\n", cursor, line))
	}
	b.WriteString("\n")
	b.WriteString("enter accept  ctrl-c cancel  j/k or arrows move")
	return b.String()
}

func inverse(value string) string {
	return "\x1b[7m" + value + "\x1b[0m"
}
