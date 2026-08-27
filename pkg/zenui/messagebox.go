package zenui

// MessageBoxConfig sets up a MessageBox.
type MessageBoxConfig struct {
	Title   string
	Message string
	// Buttons are the button labels, left to right -- arbitrary
	// choices, not limited to OK/Yes/No/Cancel. Must be non-empty; use
	// one of the purpose-specific constructors below (NewOKMessageBox
	// and friends) for the common cases instead of building this list
	// by hand.
	Buttons []string
	// CancelButtonIndex, if >= 0, is the button Escape also triggers.
	// -1 means Escape does nothing. See ModalConfig.CancelButtonIndex.
	CancelButtonIndex int
	GridSize          int
	Scale             int
}

// MessageBox is a Modal specialised to a single message and a row of
// buttons -- no other fields. Embeds *Modal directly, so Draw/Update/
// Status/Result are the same methods Modal already has; MessageBox adds
// nothing behaviourally, only a more direct way to construct the common
// "show a message, get a choice" case without building a FormConfig by
// hand.
type MessageBox struct {
	*Modal
}

// NewMessageBox creates a message box from cfg. It never returns nil.
func NewMessageBox(cfg MessageBoxConfig) *MessageBox {
	return &MessageBox{Modal: NewModal(ModalConfig{
		Form: FormConfig{
			Title:  cfg.Title,
			Fields: []FormField{{Label: cfg.Message, Type: FieldLabel}},
		},
		Buttons:           cfg.Buttons,
		CancelButtonIndex: cfg.CancelButtonIndex,
		GridSize:          cfg.GridSize,
		Scale:             cfg.Scale,
	})}
}

// NewOKMessageBox is a message box with a single "OK" button, which also
// answers Escape (CancelButtonIndex 0) -- there's only one button, so
// Escape and clicking it mean the same thing.
func NewOKMessageBox(title, message string) *MessageBox {
	return NewMessageBox(MessageBoxConfig{
		Title: title, Message: message,
		Buttons: []string{"OK"}, CancelButtonIndex: 0,
	})
}

// NewYesNoMessageBox is a message box with "No"/"Yes" buttons (in that
// order, so the default/rightmost button is "Yes", the common
// convention); Escape answers "No".
func NewYesNoMessageBox(title, message string) *MessageBox {
	return NewMessageBox(MessageBoxConfig{
		Title: title, Message: message,
		Buttons: []string{"No", "Yes"}, CancelButtonIndex: 0,
	})
}

// NewYesNoCancelMessageBox is a message box with "Cancel"/"No"/"Yes"
// buttons; Escape answers "Cancel".
func NewYesNoCancelMessageBox(title, message string) *MessageBox {
	return NewMessageBox(MessageBoxConfig{
		Title: title, Message: message,
		Buttons: []string{"Cancel", "No", "Yes"}, CancelButtonIndex: 0,
	})
}

// NewOKCancelMessageBox is a message box with "Cancel"/"OK" buttons;
// Escape answers "Cancel".
func NewOKCancelMessageBox(title, message string) *MessageBox {
	return NewMessageBox(MessageBoxConfig{
		Title: title, Message: message,
		Buttons: []string{"Cancel", "OK"}, CancelButtonIndex: 0,
	})
}

// MessageDialog is the same widget as MessageBox under a second name --
// a straight type alias, not a distinct implementation. Provided because
// different toolkit conventions call this same concept by either name;
// MessageDialog behaves identically to MessageBox in every respect,
// including sharing its Status/Result/Draw/Update via the same embedded
// *Modal. If a genuinely different visual treatment (a title bar, an
// icon) is ever wanted for one name but not the other, that's a real
// design decision to make explicitly, not something this alias
// pre-empts.
type MessageDialog = MessageBox

// MessageDialogConfig is MessageBoxConfig under MessageDialog's name.
type MessageDialogConfig = MessageBoxConfig

// NewMessageDialog is NewMessageBox under MessageDialog's name.
func NewMessageDialog(cfg MessageDialogConfig) *MessageDialog { return NewMessageBox(cfg) }

// NewOKMessageDialog is NewOKMessageBox under MessageDialog's name.
func NewOKMessageDialog(title, message string) *MessageDialog {
	return NewOKMessageBox(title, message)
}

// NewYesNoMessageDialog is NewYesNoMessageBox under MessageDialog's name.
func NewYesNoMessageDialog(title, message string) *MessageDialog {
	return NewYesNoMessageBox(title, message)
}

// NewYesNoCancelMessageDialog is NewYesNoCancelMessageBox under
// MessageDialog's name.
func NewYesNoCancelMessageDialog(title, message string) *MessageDialog {
	return NewYesNoCancelMessageBox(title, message)
}

// NewOKCancelMessageDialog is NewOKCancelMessageBox under MessageDialog's
// name.
func NewOKCancelMessageDialog(title, message string) *MessageDialog {
	return NewOKCancelMessageBox(title, message)
}
