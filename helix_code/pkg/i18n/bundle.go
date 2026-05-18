package i18n

import (
	"fmt"
	"io/fs"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

// Bundle is an opaque wrapper around go-i18n's *i18n.Bundle. It exposes only
// the subset of operations HelixCode needs and pre-registers a YAML
// unmarshaller so callers never have to know about the underlying format
// registration mechanism.
type Bundle struct {
	inner *goi18n.Bundle
	// loadedCount tracks how many message files have been registered against
	// this bundle. The Localizer consults this to surface
	// ErrBundleNotConfigured for an obviously-unconfigured bundle instead of
	// silently returning the message ID string (go-i18n's default fallback).
	loadedCount int
}

// NewBundle constructs a Bundle with the given default language tag. The
// YAML unmarshaller is registered eagerly so callers can immediately call
// LoadMessageFile / LoadMessageFileFS / MustParseMessageFileBytes on the
// returned value.
func NewBundle(defaultLang language.Tag) *Bundle {
	inner := goi18n.NewBundle(defaultLang)
	inner.RegisterUnmarshalFunc("yaml", yaml.Unmarshal)
	inner.RegisterUnmarshalFunc("yml", yaml.Unmarshal)
	return &Bundle{inner: inner}
}

// LoadMessageFile loads a YAML message file from the local filesystem.
// The file name MUST follow go-i18n's "active.<lang>.yaml" convention so the
// language tag can be inferred from the filename.
func (b *Bundle) LoadMessageFile(path string) error {
	if _, err := b.inner.LoadMessageFile(path); err != nil {
		return fmt.Errorf("i18n: load message file %q: %w", path, err)
	}
	b.loadedCount++
	return nil
}

// LoadMessageFileFS loads a YAML message file from a fs.FS (typically an
// embed.FS). Same naming convention as LoadMessageFile.
func (b *Bundle) LoadMessageFileFS(fsys fs.FS, path string) error {
	buf, err := fs.ReadFile(fsys, path)
	if err != nil {
		return fmt.Errorf("i18n: read message file %q from fs: %w", path, err)
	}
	if _, err := b.inner.ParseMessageFileBytes(buf, path); err != nil {
		return fmt.Errorf("i18n: parse message file %q from fs: %w", path, err)
	}
	b.loadedCount++
	return nil
}

// MustParseMessageFileBytes is the in-memory equivalent of LoadMessageFile,
// useful primarily for test fixtures. The path argument is parsed only for
// its language tag suffix; no I/O is performed against it. Panics on parse
// failure, matching go-i18n's upstream contract for the "Must" variant.
func (b *Bundle) MustParseMessageFileBytes(buf []byte, path string) {
	b.inner.MustParseMessageFileBytes(buf, path)
	b.loadedCount++
}

// hasLoadedFiles reports whether at least one message file has been
// registered. Localizer uses this for the ErrBundleNotConfigured guard.
func (b *Bundle) hasLoadedFiles() bool {
	return b.loadedCount > 0
}
