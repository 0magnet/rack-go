//go:build js && wasm

package rack

import (
	"syscall/js"
	"testing"
)

// A module drawn at 0.81 of its layout width measures its content a fifth
// short in screen pixels; drawnScale is what turns that back into the CSS
// pixels its width is set in. Without it, a scaled rack set every module a
// slot too narrow and cut its last column off.
func TestDrawnScaleIsDrawnOverLayoutWidth(t *testing.T) {
	el := js.Global().Call("eval", `({offsetWidth: 3000, getBoundingClientRect: function () { return {width: 2430}; }})`)
	if got := drawnScale(el); got < 0.8099 || got > 0.8101 {
		t.Errorf("drawnScale = %v, want 0.81", got)
	}
	// Nothing to tell it by: no scale, rather than a division by nothing.
	bare := js.Global().Call("eval", `({getBoundingClientRect: function () { return {width: 0}; }})`)
	if got := drawnScale(bare); got != 1 {
		t.Errorf("drawnScale of an element with no layout width = %v, want 1", got)
	}
}
