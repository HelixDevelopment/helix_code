package i18n

import (
	"errors"
	"fmt"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

// Localizer wraps a go-i18n *i18n.Localizer and routes its
// MessageNotFoundErr through ErrMessageNotFound so callers can use errors.Is
// without depending on the upstream concrete type.
type Localizer struct {
	inner  *goi18n.Localizer
	bundle *Bundle
}

// NewLocalizer constructs a Localizer for the given Bundle and accept-language
// chain. langs follows go-i18n semantics: ordered list of language preferences,
// e.g. ("sr-RS", "en"). An empty list falls back to the bundle's default
// language tag.
func NewLocalizer(b *Bundle, langs ...string) *Localizer {
	return &Localizer{
		inner:  goi18n.NewLocalizer(b.inner, langs...),
		bundle: b,
	}
}

// T resolves messageID to a localized string. Optional templateData (a single
// map[string]any) is interpolated into the message template. Returns
// ErrBundleNotConfigured if no message files have been loaded, or
// ErrMessageNotFound if the ID is unknown in every loaded locale.
func (l *Localizer) T(messageID string, templateData ...map[string]any) (string, error) {
	if !l.bundle.hasLoadedFiles() {
		return "", fmt.Errorf("i18n: cannot resolve %q: %w", messageID, ErrBundleNotConfigured)
	}
	cfg := &goi18n.LocalizeConfig{MessageID: messageID}
	if len(templateData) > 0 {
		cfg.TemplateData = templateData[0]
	}
	msg, err := l.inner.Localize(cfg)
	if err != nil {
		return "", translateNotFound(messageID, err)
	}
	return msg, nil
}

// TPlural resolves messageID using CLDR plural rules for the given count.
// PluralCount is exposed to the message template automatically, so YAML
// templates may reference {{.PluralCount}} without the caller adding it to
// templateData.
func (l *Localizer) TPlural(messageID string, count int, templateData ...map[string]any) (string, error) {
	if !l.bundle.hasLoadedFiles() {
		return "", fmt.Errorf("i18n: cannot resolve %q: %w", messageID, ErrBundleNotConfigured)
	}
	cfg := &goi18n.LocalizeConfig{
		MessageID:   messageID,
		PluralCount: count,
	}
	if len(templateData) > 0 {
		cfg.TemplateData = templateData[0]
	}
	msg, err := l.inner.Localize(cfg)
	if err != nil {
		return "", translateNotFound(messageID, err)
	}
	return msg, nil
}

// translateNotFound converts go-i18n's *i18n.MessageNotFoundErr into our
// ErrMessageNotFound sentinel, preserving the original error in the wrap chain
// so callers retain access to Tag / MessageID via errors.As.
func translateNotFound(messageID string, err error) error {
	var nf *goi18n.MessageNotFoundErr
	if errors.As(err, &nf) {
		return fmt.Errorf("i18n: %q (lang=%q): %w", nf.MessageID, nf.Tag, ErrMessageNotFound)
	}
	return fmt.Errorf("i18n: resolve %q: %w", messageID, err)
}
