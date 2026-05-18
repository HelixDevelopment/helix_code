package i18n

import "errors"

// ErrMessageNotFound is returned when a message ID is not present in the loaded
// bundle. It wraps go-i18n's *i18n.MessageNotFoundErr so callers can use
// errors.Is to test for absence without depending on the upstream concrete type.
//
// CONST-046 alignment: this sentinel is developer-facing diagnostic infrastructure,
// not user-facing content. The translated message intended for the user is the
// missing payload; surfacing this sentinel is the loud-failure signal that the
// user-facing string is undefined.
var ErrMessageNotFound = errors.New("i18n: message ID not found in bundle")

// ErrBundleNotConfigured is returned when a Localizer is asked to resolve a
// message ID but the bundle has no message files loaded at all. This is the
// explicit loud-failure for the common bring-up bug of forgetting to call
// LoadMessageFile / LoadMessageFileFS / MustParseMessageFileBytes after
// NewBundle.
var ErrBundleNotConfigured = errors.New("i18n: bundle has no message files loaded")
