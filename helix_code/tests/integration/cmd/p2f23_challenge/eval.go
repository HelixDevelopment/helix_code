package main

import "github.com/chromedp/chromedp"

// evalInputValue reads `<el>.value` for the matched selector via
// chromedp.Value. Used by PHASE-D to assert the typed text reached
// the input element (positive byte readback evidence).
func evalInputValue(selector string, out *string) chromedp.Action {
	return chromedp.Value(selector, out, chromedp.ByQuery)
}
