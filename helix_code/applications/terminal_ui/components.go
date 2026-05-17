package main

import (
	"fmt"

	"github.com/rivo/tview"
)

// UIComponents contains reusable UI components for the Terminal UI
type UIComponents struct {
	tui *TerminalUI
}

// NewUIComponents creates a new UI components instance
func NewUIComponents(tui *TerminalUI) *UIComponents {
	return &UIComponents{
		tui: tui,
	}
}

// CreateForm creates a reusable form component. Each FormField is
// dispatched on its Type into the matching tview widget — "input",
// "password", "checkbox", "dropdown", "button". Fields with empty
// Type default to "input". Fields with Type "button" become form
// buttons; all other types become input widgets. Unknown Type
// strings panic-free: the field is skipped and a debug entry is
// logged via the form title (no silent ignore — the previous
// implementation ignored every field, which was a §11.4 stub bluff).
func (uc *UIComponents) CreateForm(title string, fields []FormField) *tview.Form {
	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(title)
	form.SetTitleAlign(tview.AlignLeft)

	if len(fields) == 0 {
		// No fields configured — caller intentionally wants an empty
		// form. Do NOT silently inject a generic "Input" widget; the
		// previous behavior masked configuration errors.
		return form
	}

	for _, f := range fields {
		width := f.Width
		if width <= 0 {
			width = 20
		}
		switch f.Type {
		case "", "input", "text":
			form.AddInputField(f.Label, f.DefaultValue, width, nil, asStringChange(f.OnChange))
		case "password":
			form.AddPasswordField(f.Label, f.DefaultValue, width, '*', asStringChange(f.OnChange))
		case "checkbox", "bool":
			form.AddCheckbox(f.Label, f.BoolValue, asBoolChange(f.OnChange))
		case "dropdown", "select":
			form.AddDropDown(f.Label, f.Options, f.SelectedIndex, asSelectChange(f.OnChange))
		case "button":
			form.AddButton(f.Label, f.OnClick)
		default:
			// Unknown type — surface in the form title so the misuse
			// is visible to the user (anti-bluff: no silent skips).
			form.SetTitle(fmt.Sprintf("%s [skipped unknown field type %q]", title, f.Type))
		}
	}

	return form
}

// asStringChange casts an interface{} OnChange handler into the
// signature tview.Form.AddInputField expects. Returns nil if the
// handler is nil or the wrong type.
func asStringChange(h interface{}) func(string) {
	if fn, ok := h.(func(string)); ok {
		return fn
	}
	return nil
}

// asBoolChange casts an interface{} OnChange handler for checkboxes.
func asBoolChange(h interface{}) func(bool) {
	if fn, ok := h.(func(bool)); ok {
		return fn
	}
	return nil
}

// asSelectChange casts an interface{} OnChange handler for dropdowns.
func asSelectChange(h interface{}) func(string, int) {
	if fn, ok := h.(func(string, int)); ok {
		return fn
	}
	return nil
}

// CreateList creates a reusable list component
func (uc *UIComponents) CreateList(title string, items []ListItem) *tview.List {
	list := tview.NewList()
	list.SetBorder(true)
	list.SetTitle(title)
	list.SetTitleAlign(tview.AlignLeft)

	for _, item := range items {
		list.AddItem(item.MainText, item.SecondaryText, item.Shortcut, item.OnSelect)
	}

	return list
}

// CreateTable creates a reusable table component
func (uc *UIComponents) CreateTable(title string, headers []string, data [][]string) *tview.Table {
	table := tview.NewTable()
	table.SetBorder(true)
	table.SetTitle(title)
	table.SetTitleAlign(tview.AlignLeft)

	// Add headers
	for col, header := range headers {
		table.SetCell(0, col, tview.NewTableCell(header).
			SetTextColor(tview.Styles.SecondaryTextColor).
			SetSelectable(false))
	}

	// Add data
	for row, rowData := range data {
		for col, cellData := range rowData {
			table.SetCell(row+1, col, tview.NewTableCell(cellData))
		}
	}

	return table
}

// CreateProgressBar creates a progress bar component
func (uc *UIComponents) CreateProgressBar(title string, current, total int) *tview.TextView {
	progress := tview.NewTextView()
	progress.SetBorder(true)
	progress.SetTitle(title)
	progress.SetTitleAlign(tview.AlignLeft)
	progress.SetDynamicColors(true)

	percentage := float64(current) / float64(total) * 100
	progress.SetText(uc.formatProgressBar(current, total, percentage))

	return progress
}

// CreateModal creates a modal dialog
func (uc *UIComponents) CreateModal(title, message string, buttons []ModalButton) *tview.Modal {
	modal := tview.NewModal()
	modal.SetText(message)
	modal.SetBackgroundColor(tview.Styles.ContrastBackgroundColor)

	buttonLabels := make([]string, len(buttons))
	for i, button := range buttons {
		buttonLabels[i] = button.Label
	}
	modal.AddButtons(buttonLabels)

	// Handle button clicks
	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		if buttonIndex >= 0 && buttonIndex < len(buttons) {
			buttons[buttonIndex].OnClick()
		}
	})

	return modal
}

// CreateStatusBar creates a status bar component
func (uc *UIComponents) CreateStatusBar() *tview.TextView {
	status := tview.NewTextView()
	status.SetDynamicColors(true)
	status.SetTextAlign(tview.AlignCenter)
	status.SetText("[green]Ready")

	status.SetBorder(true)
	status.SetTitle("Status")
	return status
}

// CreateLogView creates a log/scrollable text view
func (uc *UIComponents) CreateLogView(title string) *tview.TextView {
	logView := tview.NewTextView()
	logView.SetBorder(true)
	logView.SetTitle(title)
	logView.SetTitleAlign(tview.AlignLeft)
	logView.SetDynamicColors(true)
	logView.SetScrollable(true)
	logView.SetWrap(true)

	return logView
}

// Helper functions

// formatProgressBar formats a progress bar string
func (uc *UIComponents) formatProgressBar(current, total int, percentage float64) string {
	width := 20
	filled := int(float64(width) * percentage / 100)

	bar := ""
	for i := 0; i < width; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}

	return fmt.Sprintf("[green]%s[white] %d/%d (%.1f%%)", bar, current, total, percentage)
}

// FormField represents a form field configuration
type FormField struct {
	Type          string
	Label         string
	DefaultValue  string
	Width         int
	BoolValue     bool
	Options       []string
	SelectedIndex int
	OnChange      interface{}
	OnClick       func()
}

// ListItem represents a list item configuration
type ListItem struct {
	MainText      string
	SecondaryText string
	Shortcut      rune
	OnSelect      func()
}

// ModalButton represents a modal button configuration
type ModalButton struct {
	Label   string
	OnClick func()
}
