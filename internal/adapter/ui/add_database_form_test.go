package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// ==========================================
// Helper Functions
// ==========================================

// makeKeyPress creates a tea.KeyPressMsg for special keys.
func makeKeyPress(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Text: "", Mod: 0}
}

// makeCharPress creates a tea.KeyPressMsg for character input.
func makeCharPress(char string) tea.KeyPressMsg {
	runes := []rune(char)
	code := ' '
	if len(runes) > 0 {
		code = runes[0]
	}
	return tea.KeyPressMsg{Code: code, Text: char, Mod: 0}
}

// makeCtrlKeyPress creates a tea.KeyPressMsg for ctrl+key combinations.
// Note: key should be lowercase (e.g., 'u' not 'U') for proper string matching.
func makeCtrlKeyPress(key rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: key, Text: "", Mod: tea.ModCtrl}
}

// makeShiftKeyPress creates a tea.KeyPressMsg for shift+key combinations.
func makeShiftKeyPress(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Text: "", Mod: tea.ModShift}
}

// ==========================================
// AddDatabaseForm Creation Tests
// ==========================================

func TestNewAddDatabaseFormCreatesCorrectFieldCount(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)

	if len(form.fields) != formFieldCount {
		t.Errorf("expected %d fields, got %d", formFieldCount, len(form.fields))
	}
	if formFieldCount != 6 {
		t.Errorf("expected formFieldCount to be 6, got %d", formFieldCount)
	}
}

func TestNewAddDatabaseFormInitialState(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(100, 40)

	if form.cursor != 0 {
		t.Errorf("expected initial cursor to be 0, got %d", form.cursor)
	}
	if form.width != 100 {
		t.Errorf("expected width to be 100, got %d", form.width)
	}
	if form.height != 40 {
		t.Errorf("expected height to be 40, got %d", form.height)
	}
	if form.submitted {
		t.Error("expected submitted to be false initially")
	}
	if form.cancelled {
		t.Error("expected cancelled to be false initially")
	}
	if form.errMsg != "" {
		t.Errorf("expected empty error message initially, got %q", form.errMsg)
	}
}

func TestNewAddDatabaseFormFieldLabels(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)

	expectedLabels := []string{
		"Database ID",
		"Database Host",
		"Database Port",
		"Service Name / SID",
		"Username",
		"Password",
	}

	for i, expected := range expectedLabels {
		if form.fields[i].Label != expected {
			t.Errorf("field[%d] label = %q, want %q", i, form.fields[i].Label, expected)
		}
	}
}

func TestNewAddDatabaseFormPasswordField(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)

	// Only the Password field (index 5) should be a password field
	for i, field := range form.fields {
		if i == formFieldPass {
			if !field.IsPassword {
				t.Errorf("field[%d] (%s) expected IsPassword=true, got false", i, field.Label)
			}
		} else {
			if field.IsPassword {
				t.Errorf("field[%d] (%s) expected IsPassword=false, got true", i, field.Label)
			}
		}
	}
}

// ==========================================
// FieldValues Tests
// ==========================================

func TestFieldValuesReturnsCorrectValues(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)

	// Set values by simulating input
	form.fields[formFieldDatabaseID].Value = "TEST-DB"
	form.fields[formFieldHost].Value = "localhost"
	form.fields[formFieldPort].Value = "1521"
	form.fields[formFieldService].Value = "FREEPDB1"
	form.fields[formFieldUser].Value = "SYSTEM"
	form.fields[formFieldPass].Value = "secret"

	dbID, host, port, service, user, pass := form.FieldValues()

	if dbID != "TEST-DB" {
		t.Errorf("databaseID = %q, want %q", dbID, "TEST-DB")
	}
	if host != "localhost" {
		t.Errorf("host = %q, want %q", host, "localhost")
	}
	if port != "1521" {
		t.Errorf("port = %q, want %q", port, "1521")
	}
	if service != "FREEPDB1" {
		t.Errorf("service = %q, want %q", service, "FREEPDB1")
	}
	if user != "SYSTEM" {
		t.Errorf("user = %q, want %q", user, "SYSTEM")
	}
	if pass != "secret" {
		t.Errorf("password = %q, want %q", pass, "secret")
	}
}

// ==========================================
// WithDimensions Tests
// ==========================================

func TestWithDimensionsUpdatesDimensions(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.fields[formFieldDatabaseID].Value = "TEST-DB"

	updated := form.WithDimensions(120, 40)

	if updated.width != 120 {
		t.Errorf("width = %d, want 120", updated.width)
	}
	if updated.height != 40 {
		t.Errorf("height = %d, want 40", updated.height)
	}
}

func TestWithDimensionsPreservesFieldValues(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.fields[formFieldDatabaseID].Value = "PRESERVE-ME"
	form.fields[formFieldHost].Value = "myhost"
	form.cursor = 3

	updated := form.WithDimensions(100, 50)

	if updated.fields[formFieldDatabaseID].Value != "PRESERVE-ME" {
		t.Errorf("databaseID value was not preserved: got %q", updated.fields[formFieldDatabaseID].Value)
	}
	if updated.fields[formFieldHost].Value != "myhost" {
		t.Errorf("host value was not preserved: got %q", updated.fields[formFieldHost].Value)
	}
	if updated.cursor != 3 {
		t.Errorf("cursor was not preserved: got %d", updated.cursor)
	}
}

// ==========================================
// IsSubmitted / IsCancelled Tests
// ==========================================

func TestIsSubmittedInitialState(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)

	if form.IsSubmitted() {
		t.Error("expected IsSubmitted() to be false initially")
	}
}

func TestIsCancelledInitialState(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)

	if form.IsCancelled() {
		t.Error("expected IsCancelled() to be false initially")
	}
}

// ==========================================
// Navigation Tests
// ==========================================

func TestUpNavigationCyclesUp(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.cursor = 3

	updated, _ := form.Update(makeKeyPress(tea.KeyUp))

	if updated.cursor != 2 {
		t.Errorf("cursor = %d, want 2", updated.cursor)
	}
}

func TestUpNavigationStopsAtZero(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.cursor = 0

	updated, _ := form.Update(makeKeyPress(tea.KeyUp))

	if updated.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (should stop at first position)", updated.cursor)
	}
}

func TestDownNavigationCyclesDown(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.cursor = 2

	updated, _ := form.Update(makeKeyPress(tea.KeyDown))

	if updated.cursor != 3 {
		t.Errorf("cursor = %d, want 3", updated.cursor)
	}
}

func TestDownNavigationStopsAtMaxCursor(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.cursor = formMaxCursor

	updated, _ := form.Update(makeKeyPress(tea.KeyDown))

	if updated.cursor != formMaxCursor {
		t.Errorf("cursor = %d, want %d (should stop at max)", updated.cursor, formMaxCursor)
	}
}

func TestTabNavigationCyclesThroughAllPositions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		startCursor  int
		expectedNext int
	}{
		{"from field 0 to 1", 0, 1},
		{"from field 5 to save", 5, formBtnSave},
		{"from save to cancel", formBtnSave, formBtnCancel},
		{"from cancel back to 0", formBtnCancel, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form := NewAddDatabaseForm(80, 30)
			form.cursor = tt.startCursor

			updated, _ := form.Update(makeKeyPress(tea.KeyTab))

			if updated.cursor != tt.expectedNext {
				t.Errorf("cursor = %d, want %d", updated.cursor, tt.expectedNext)
			}
		})
	}
}

func TestShiftTabNavigation(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.cursor = 1

	updated, _ := form.Update(makeShiftKeyPress(tea.KeyTab))

	if updated.cursor != 0 {
		t.Errorf("cursor = %d, want 0 after shift+tab from position 1", updated.cursor)
	}
}

func TestShiftTabStopsAtZero(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.cursor = 0

	updated, _ := form.Update(makeShiftKeyPress(tea.KeyTab))

	if updated.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (shift+tab should not go below 0)", updated.cursor)
	}
}

// ==========================================
// Enter Key Tests
// ==========================================

func TestEnterOnFieldAdvancesCursor(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.cursor = 2 // Port field

	updated, _ := form.Update(makeKeyPress(tea.KeyEnter))

	if updated.cursor != 3 {
		t.Errorf("cursor = %d, want 3 after enter on field 2", updated.cursor)
	}
}

func TestEnterOnLastFieldAdvancesToSave(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.cursor = formFieldCount - 1 // Last field (password)

	updated, _ := form.Update(makeKeyPress(tea.KeyEnter))

	if updated.cursor != formBtnSave {
		t.Errorf("cursor = %d, want %d (save button) after enter on last field", updated.cursor, formBtnSave)
	}
}

func TestEnterOnSaveWithInvalidDataSetsError(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.cursor = formBtnSave
	// All fields are empty - validation should fail

	updated, _ := form.Update(makeKeyPress(tea.KeyEnter))

	if updated.submitted {
		t.Error("expected submitted to remain false with invalid data")
	}
	if updated.errMsg == "" {
		t.Error("expected error message to be set for invalid data")
	}
}

func TestEnterOnSaveWithValidDataSetsSubmitted(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.cursor = formBtnSave
	// Fill all required fields with valid data
	form.fields[formFieldDatabaseID].Value = "TEST-DB"
	form.fields[formFieldHost].Value = "localhost"
	form.fields[formFieldPort].Value = "1521"
	form.fields[formFieldService].Value = "FREEPDB1"
	form.fields[formFieldUser].Value = "SYSTEM"
	form.fields[formFieldPass].Value = "password"

	updated, _ := form.Update(makeKeyPress(tea.KeyEnter))

	if !updated.submitted {
		t.Error("expected submitted to be true with valid data")
	}
	if updated.errMsg != "" {
		t.Errorf("expected no error message with valid data, got %q", updated.errMsg)
	}
}

func TestEnterOnCancelSetsCancelled(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.cursor = formBtnCancel

	updated, _ := form.Update(makeKeyPress(tea.KeyEnter))

	if !updated.cancelled {
		t.Error("expected cancelled to be true after enter on cancel button")
	}
}

func TestEscapeSetsCancelled(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.cursor = 2

	updated, _ := form.Update(makeKeyPress(tea.KeyEsc))

	if !updated.cancelled {
		t.Error("expected cancelled to be true after esc key")
	}
}

// ==========================================
// Character Input Tests
// ==========================================

func TestCharacterInputAppendsToField(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.cursor = 0

	updated, _ := form.Update(makeCharPress("A"))

	if updated.fields[0].Value != "A" {
		t.Errorf("field value = %q, want %q", updated.fields[0].Value, "A")
	}
}

func TestCharacterInputAppendsMultipleCharacters(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.cursor = 0
	form.fields[0].Value = "TEST"

	updated, _ := form.Update(makeCharPress("DB"))

	if updated.fields[0].Value != "TESTDB" {
		t.Errorf("field value = %q, want %q", updated.fields[0].Value, "TESTDB")
	}
}

func TestCharacterInputOnlyOnFields(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.cursor = formBtnSave // On button, not a field

	updated, _ := form.Update(makeCharPress("X"))

	if updated.fields[0].Value != "" {
		t.Error("character input should not affect fields when cursor is on button")
	}
}

func TestCtrlModifierBlocksCharacterInput(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.cursor = 0

	updated, _ := form.Update(makeCtrlKeyPress('A'))

	if updated.fields[0].Value != "" {
		t.Error("character input with ctrl modifier should be ignored")
	}
}

// ==========================================
// Backspace Tests
// ==========================================

func TestBackspaceRemovesLastCharacter(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.cursor = 0
	form.fields[0].Value = "TEST"

	updated, _ := form.Update(makeKeyPress(tea.KeyBackspace))

	if updated.fields[0].Value != "TES" {
		t.Errorf("field value = %q, want %q", updated.fields[0].Value, "TES")
	}
}

func TestBackspaceOnEmptyFieldDoesNothing(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.cursor = 0
	form.fields[0].Value = ""

	updated, _ := form.Update(makeKeyPress(tea.KeyBackspace))

	if updated.fields[0].Value != "" {
		t.Error("backspace on empty field should do nothing")
	}
}

func TestBackspaceOnButtonsDoesNothing(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.cursor = formBtnSave
	form.fields[0].Value = "TEST"

	updated, _ := form.Update(makeKeyPress(tea.KeyBackspace))

	// Field value should be unchanged since cursor is on button
	if updated.fields[0].Value != "TEST" {
		t.Errorf("backspace should not affect fields when cursor is on button, got %q", updated.fields[0].Value)
	}
}

// ==========================================
// Ctrl+U Clear Field Tests
// ==========================================

func TestCtrlUClearsCurrentField(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.cursor = 0
	form.fields[0].Value = "TEST-DB"

	// Use lowercase 'u' for ctrl+u matching
	updated, _ := form.Update(makeCtrlKeyPress('u'))

	if updated.fields[0].Value != "" {
		t.Errorf("field value = %q, want empty string", updated.fields[0].Value)
	}
}

func TestCtrlUOnButtonsDoesNothing(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.cursor = formBtnCancel
	form.fields[0].Value = "TEST"

	// Use lowercase 'u' for ctrl+u matching
	updated, _ := form.Update(makeCtrlKeyPress('u'))

	if updated.fields[0].Value != "TEST" {
		t.Error("ctrl+u should not affect fields when cursor is on button")
	}
}

// ==========================================
// Paste Tests
// ==========================================

func TestPasteAppendsToCurrentField(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.cursor = 0
	form.fields[0].Value = "TEST"

	updated, _ := form.Update(tea.PasteMsg{Content: "DB"})

	if updated.fields[0].Value != "TESTDB" {
		t.Errorf("field value = %q, want %q", updated.fields[0].Value, "TESTDB")
	}
}

func TestPasteSanitizesNewlines(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.cursor = 0

	updated, _ := form.Update(tea.PasteMsg{Content: "TEST\nDB"})

	if updated.fields[0].Value != "TESTDB" {
		t.Errorf("field value = %q, want %q (newlines should be removed)", updated.fields[0].Value, "TESTDB")
	}
}

func TestPasteSanitizesCarriageReturn(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.cursor = 0

	updated, _ := form.Update(tea.PasteMsg{Content: "TEST\rDB"})

	if updated.fields[0].Value != "TESTDB" {
		t.Errorf("field value = %q, want %q (carriage returns should be removed)", updated.fields[0].Value, "TESTDB")
	}
}

func TestPasteSanitizesTabs(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.cursor = 0

	updated, _ := form.Update(tea.PasteMsg{Content: "TEST\tDB"})

	if updated.fields[0].Value != "TESTDB" {
		t.Errorf("field value = %q, want %q (tabs should be removed)", updated.fields[0].Value, "TESTDB")
	}
}

func TestPasteSanitizesControlCharacters(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.cursor = 0

	// NUL (0x00), BEL (0x07), etc. are control characters
	updated, _ := form.Update(tea.PasteMsg{Content: "TEST\x00DB\x07"})

	if updated.fields[0].Value != "TESTDB" {
		t.Errorf("field value = %q, want %q (control characters should be removed)", updated.fields[0].Value, "TESTDB")
	}
}

func TestPasteIgnoredOnButtons(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.cursor = formBtnSave
	form.fields[0].Value = "TEST"

	updated, _ := form.Update(tea.PasteMsg{Content: "PASTED"})

	// Field value should be unchanged since cursor is on button
	if updated.fields[0].Value != "TEST" {
		t.Errorf("paste should not affect fields when cursor is on button, got %q", updated.fields[0].Value)
	}
}

func TestPasteClearsErrorMessage(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.errMsg = "Previous error"

	updated, _ := form.Update(tea.PasteMsg{Content: "paste"})

	if updated.errMsg != "" {
		t.Errorf("expected error message to be cleared after paste, got %q", updated.errMsg)
	}
}

// ==========================================
// Validation Tests
// ==========================================

func TestValidateEmptyDatabaseID(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.fields[formFieldDatabaseID].Value = ""
	form.fields[formFieldHost].Value = "localhost"
	form.fields[formFieldPort].Value = "1521"
	form.fields[formFieldService].Value = "FREEPDB1"
	form.fields[formFieldUser].Value = "SYSTEM"
	form.fields[formFieldPass].Value = "password"

	err := form.validate()

	if err != "Database ID cannot be empty" {
		t.Errorf("expected error about empty Database ID, got %q", err)
	}
}

func TestValidateEmptyHost(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.fields[formFieldDatabaseID].Value = "TEST-DB"
	form.fields[formFieldHost].Value = ""
	form.fields[formFieldPort].Value = "1521"
	form.fields[formFieldService].Value = "FREEPDB1"
	form.fields[formFieldUser].Value = "SYSTEM"
	form.fields[formFieldPass].Value = "password"

	err := form.validate()

	if err != "Database host cannot be empty" {
		t.Errorf("expected error about empty host, got %q", err)
	}
}

func TestValidateEmptyPort(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.fields[formFieldDatabaseID].Value = "TEST-DB"
	form.fields[formFieldHost].Value = "localhost"
	form.fields[formFieldPort].Value = ""
	form.fields[formFieldService].Value = "FREEPDB1"
	form.fields[formFieldUser].Value = "SYSTEM"
	form.fields[formFieldPass].Value = "password"

	err := form.validate()

	if err != "Database port cannot be empty" {
		t.Errorf("expected error about empty port, got %q", err)
	}
}

func TestValidateInvalidPortNonNumeric(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.fields[formFieldDatabaseID].Value = "TEST-DB"
	form.fields[formFieldHost].Value = "localhost"
	form.fields[formFieldPort].Value = "abc"
	form.fields[formFieldService].Value = "FREEPDB1"
	form.fields[formFieldUser].Value = "SYSTEM"
	form.fields[formFieldPass].Value = "password"

	err := form.validate()

	if err != "Port must be a number between 1 and 65535" {
		t.Errorf("expected error about invalid port, got %q", err)
	}
}

func TestValidateInvalidPortZero(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.fields[formFieldDatabaseID].Value = "TEST-DB"
	form.fields[formFieldHost].Value = "localhost"
	form.fields[formFieldPort].Value = "0"
	form.fields[formFieldService].Value = "FREEPDB1"
	form.fields[formFieldUser].Value = "SYSTEM"
	form.fields[formFieldPass].Value = "password"

	err := form.validate()

	if err != "Port must be a number between 1 and 65535" {
		t.Errorf("expected error about invalid port (0), got %q", err)
	}
}

func TestValidateInvalidPortNegative(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.fields[formFieldDatabaseID].Value = "TEST-DB"
	form.fields[formFieldHost].Value = "localhost"
	form.fields[formFieldPort].Value = "-1"
	form.fields[formFieldService].Value = "FREEPDB1"
	form.fields[formFieldUser].Value = "SYSTEM"
	form.fields[formFieldPass].Value = "password"

	err := form.validate()

	if err != "Port must be a number between 1 and 65535" {
		t.Errorf("expected error about invalid port (-1), got %q", err)
	}
}

func TestValidateInvalidPortTooHigh(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.fields[formFieldDatabaseID].Value = "TEST-DB"
	form.fields[formFieldHost].Value = "localhost"
	form.fields[formFieldPort].Value = "65536"
	form.fields[formFieldService].Value = "FREEPDB1"
	form.fields[formFieldUser].Value = "SYSTEM"
	form.fields[formFieldPass].Value = "password"

	err := form.validate()

	if err != "Port must be a number between 1 and 65535" {
		t.Errorf("expected error about invalid port (65536), got %q", err)
	}
}

func TestValidateValidPortBoundaryMin(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.fields[formFieldDatabaseID].Value = "TEST-DB"
	form.fields[formFieldHost].Value = "localhost"
	form.fields[formFieldPort].Value = "1"
	form.fields[formFieldService].Value = "FREEPDB1"
	form.fields[formFieldUser].Value = "SYSTEM"
	form.fields[formFieldPass].Value = "password"

	err := form.validate()

	if err != "" {
		t.Errorf("expected no error for valid port 1, got %q", err)
	}
}

func TestValidateValidPortBoundaryMax(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.fields[formFieldDatabaseID].Value = "TEST-DB"
	form.fields[formFieldHost].Value = "localhost"
	form.fields[formFieldPort].Value = "65535"
	form.fields[formFieldService].Value = "FREEPDB1"
	form.fields[formFieldUser].Value = "SYSTEM"
	form.fields[formFieldPass].Value = "password"

	err := form.validate()

	if err != "" {
		t.Errorf("expected no error for valid port 65535, got %q", err)
	}
}

func TestValidateEmptyService(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.fields[formFieldDatabaseID].Value = "TEST-DB"
	form.fields[formFieldHost].Value = "localhost"
	form.fields[formFieldPort].Value = "1521"
	form.fields[formFieldService].Value = ""
	form.fields[formFieldUser].Value = "SYSTEM"
	form.fields[formFieldPass].Value = "password"

	err := form.validate()

	if err != "Service name cannot be empty" {
		t.Errorf("expected error about empty service name, got %q", err)
	}
}

func TestValidateEmptyUsername(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.fields[formFieldDatabaseID].Value = "TEST-DB"
	form.fields[formFieldHost].Value = "localhost"
	form.fields[formFieldPort].Value = "1521"
	form.fields[formFieldService].Value = "FREEPDB1"
	form.fields[formFieldUser].Value = ""
	form.fields[formFieldPass].Value = "password"

	err := form.validate()

	if err != "Username cannot be empty" {
		t.Errorf("expected error about empty username, got %q", err)
	}
}

func TestValidateEmptyPassword(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.fields[formFieldDatabaseID].Value = "TEST-DB"
	form.fields[formFieldHost].Value = "localhost"
	form.fields[formFieldPort].Value = "1521"
	form.fields[formFieldService].Value = "FREEPDB1"
	form.fields[formFieldUser].Value = "SYSTEM"
	form.fields[formFieldPass].Value = ""

	err := form.validate()

	if err != "Password cannot be empty" {
		t.Errorf("expected error about empty password, got %q", err)
	}
}

func TestValidateAllFieldsValid(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.fields[formFieldDatabaseID].Value = "TEST-DB"
	form.fields[formFieldHost].Value = "localhost"
	form.fields[formFieldPort].Value = "1521"
	form.fields[formFieldService].Value = "FREEPDB1"
	form.fields[formFieldUser].Value = "SYSTEM"
	form.fields[formFieldPass].Value = "password"

	err := form.validate()

	if err != "" {
		t.Errorf("expected no error for valid form, got %q", err)
	}
}

func TestValidateWhitespaceOnlyFields(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.fields[formFieldDatabaseID].Value = "   "
	form.fields[formFieldHost].Value = "   "
	form.fields[formFieldPort].Value = "   "
	form.fields[formFieldService].Value = "   "
	form.fields[formFieldUser].Value = "   "
	form.fields[formFieldPass].Value = "   "

	err := form.validate()

	// All whitespace-only fields should be treated as empty
	if err == "" {
		t.Error("expected validation error for whitespace-only fields")
	}
}

// ==========================================
// Submitted/Cancelled State Tests
// ==========================================

func TestUpdateAfterSubmittedReturnsSameState(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.submitted = true
	form.fields[0].Value = "TEST"

	updated, cmd := form.Update(makeCharPress("X"))

	// Should return same state without changes
	if updated.fields[0].Value != "TEST" {
		t.Errorf("field value should not change after submitted, got %q", updated.fields[0].Value)
	}
	if cmd != nil {
		t.Error("expected no command when already submitted")
	}
}

func TestUpdateAfterCancelledReturnsSameState(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.cancelled = true
	form.fields[0].Value = "TEST"

	updated, cmd := form.Update(makeCharPress("X"))

	// Should return same state without changes
	if updated.fields[0].Value != "TEST" {
		t.Errorf("field value should not change after cancelled, got %q", updated.fields[0].Value)
	}
	if cmd != nil {
		t.Error("expected no command when already cancelled")
	}
}

func TestErrorClearedOnNavigation(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)
	form.errMsg = "Some error"

	updated, _ := form.Update(makeKeyPress(tea.KeyUp))

	if updated.errMsg != "" {
		t.Errorf("expected error to be cleared on navigation, got %q", updated.errMsg)
	}
}

// ==========================================
// Render Tests
// ==========================================

func TestRenderReturnsNonEmptyString(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)

	rendered := form.Render()

	if rendered == "" {
		t.Error("expected Render() to return non-empty string")
	}
}

func TestModalReturnsNonEmptyString(t *testing.T) {
	t.Parallel()

	form := NewAddDatabaseForm(80, 30)

	modal := form.Modal()

	if modal == "" {
		t.Error("expected Modal() to return non-empty string")
	}
}
