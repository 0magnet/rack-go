//go:build js && wasm

package rack

import "testing"

// Release has to take the rack's listeners off the window, not only free
// them. The window outlives the rack, so a freed function left attached there
// is called on every pointer event afterwards — "call to released function"
// once per rack that was ever discarded.
func TestReleaseDetachesWindowListeners(t *testing.T) {
	if realDOM() {
		t.Skip("counts listeners on the fake window")
	}
	mk := installFakeDOM()
	listeners := func() int { return window.Get("_listeners").Length() }
	before := listeners()
	r := New(Options{Container: mk("div")})
	if listeners() == before {
		t.Fatal("a reorderable rack put no listeners on the window; the test is not testing anything")
	}
	r.Release()
	if got := listeners(); got != before {
		t.Fatalf("%d window listeners after Release, want %d", got, before)
	}
}
