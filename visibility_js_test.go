//go:build js && wasm

package rack

import (
	"strings"
	"syscall/js"
	"testing"
)

// newRack builds a rack holding modules with the given header names.
func newRack(t *testing.T, opts Options, names ...string) (*Rack, func(string) js.Value) {
	t.Helper()
	mk := installFakeDOM()
	if !opts.Container.Truthy() {
		opts.Container = mk("div")
	}
	opts.Fixed = true // no drag wiring; these tests are about state, not pointers
	r := New(opts)
	for _, n := range names {
		r.Root().Call("appendChild", module(mk, n))
	}
	return r, mk
}

func TestModulesAndKeys(t *testing.T) {
	r, _ := newRack(t, Options{}, "Osc", "Filter", "VCA")
	if got := len(r.Modules()); got != 3 {
		t.Fatalf("got %d modules, want 3", got)
	}
	want := []string{"osc", "filter", "vca"}
	got := r.Order()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("Order = %v, want %v", got, want)
	}
}

// Keys are normalized, so the header's capitalization and padding do not
// change what a saved rack refers to.
func TestKeysAreNormalized(t *testing.T) {
	r, _ := newRack(t, Options{}, "  MiXeD Case  ")
	got := r.Order()
	if len(got) != 1 {
		t.Fatalf("got %v", got)
	}
	if got[0] != strings.ToLower(strings.TrimSpace("MiXeD Case")) {
		t.Errorf("key = %q, want the header lowercased and trimmed", got[0])
	}
}

// An element in the container that is not a module must be left out, or a
// decoration counts as a rack module.
func TestModulesIgnoresElementsWithoutTheModuleClass(t *testing.T) {
	r, mk := newRack(t, Options{}, "Osc")
	r.Root().Call("appendChild", mk("div")) // no rack-mod class
	if got := len(r.Modules()); got != 1 {
		t.Errorf("got %d modules, want 1 — a plain element was counted", got)
	}
}

func TestHideAndShow(t *testing.T) {
	r, _ := newRack(t, Options{}, "Osc", "Filter")

	if r.Hidden("osc") {
		t.Fatal("a module was hidden before anything hid it")
	}
	if !r.Hide("osc") {
		t.Error("Hide reported that it did nothing")
	}
	if !r.Hidden("osc") {
		t.Error("the module is not hidden after Hide")
	}
	if r.Hide("osc") {
		t.Error("hiding an already hidden module reported a change")
	}
	if !r.Show("osc") {
		t.Error("Show reported that it did nothing")
	}
	if r.Hidden("osc") {
		t.Error("the module is still hidden after Show")
	}
	if r.Show("osc") {
		t.Error("showing an already visible module reported a change")
	}
}

// Toggle reports whether the module is now shown, not whether it changed.
func TestToggle(t *testing.T) {
	r, _ := newRack(t, Options{}, "Osc")
	if r.Toggle("osc") {
		t.Error("the first toggle reported the module still shown")
	}
	if !r.Hidden("osc") {
		t.Error("the first toggle did not hide the module")
	}
	if !r.Toggle("osc") {
		t.Error("the second toggle did not report the module shown")
	}
	if r.Hidden("osc") {
		t.Error("the second toggle did not show it again")
	}
}

// A pinned module cannot be hidden, so toggling one reports it still shown
// rather than claiming a change that did not happen.
func TestTogglePinnedReportsStillShown(t *testing.T) {
	r, _ := newRack(t, Options{
		Pinned: func(key string) bool { return key == "master" },
	}, "Master")
	if !r.Toggle("master") {
		t.Error("toggling a pinned module reported it hidden")
	}
}

// A pinned module is the way out of an emptied rack, so it must not be
// possible to put it away.
func TestPinnedModulesCannotBeHidden(t *testing.T) {
	r, _ := newRack(t, Options{
		Pinned: func(key string) bool { return key == "master" },
	}, "Osc", "Master")

	if r.Hide("master") {
		t.Error("Hide reported hiding a pinned module")
	}
	if r.Hidden("master") {
		t.Error("a pinned module was hidden")
	}
	if !r.Hide("osc") {
		t.Error("an unpinned module could not be hidden")
	}
}

func TestPinnedModulesCannotBeToggledAway(t *testing.T) {
	r, _ := newRack(t, Options{
		Pinned: func(key string) bool { return key == "master" },
	}, "Master")
	r.Toggle("master")
	if r.Hidden("master") {
		t.Error("Toggle put away a pinned module")
	}
}

// The host is told about visibility changes, and only about real ones.
func TestOnVisibilityIsCalledForRealChangesOnly(t *testing.T) {
	var events []string
	r, _ := newRack(t, Options{
		OnVisibility: func(key string, visible bool) {
			events = append(events, key+":"+map[bool]string{true: "show", false: "hide"}[visible])
		},
	}, "Osc")

	r.Hide("osc")
	r.Hide("osc") // no change
	r.Show("osc")
	r.Show("osc") // no change

	want := "osc:hide,osc:show"
	if got := strings.Join(events, ","); got != want {
		t.Errorf("events = %q, want %q", got, want)
	}
}

func TestHiddenKeysReportsWhatIsPutAway(t *testing.T) {
	r, _ := newRack(t, Options{}, "Osc", "Filter", "VCA")
	r.Hide("osc")
	r.Hide("vca")

	got := map[string]bool{}
	for _, k := range r.HiddenKeys() {
		got[k] = true
	}
	if len(got) != 2 || !got["osc"] || !got["vca"] {
		t.Errorf("HiddenKeys = %v, want osc and vca", r.HiddenKeys())
	}
}

// A key naming a module that has not been built yet is kept, or the module
// would appear the moment it is created despite having been put away.
func TestSetHiddenKeepsKeysForModulesNotPresent(t *testing.T) {
	r, _ := newRack(t, Options{}, "Osc")
	r.SetHidden([]string{"osc", "notbuiltyet"})

	if !r.Hidden("osc") {
		t.Error("a restored key for a present module did not take")
	}
	if !r.Hidden("notbuiltyet") {
		t.Error("a restored key for an absent module was forgotten")
	}
}

// Restoring a set that no longer contains a key has to show that module again,
// element and all: Apply only ever hides, so nothing else would undo the
// display it was given when it was put away.
//
// SetHidden does not call OnVisibility. It is a bulk restore the host asked
// for, and telling the host about each key it just supplied would invite a
// save-restore loop. Show is the call that reports.
func TestSetHiddenShowsWhatIsNoLongerInTheSet(t *testing.T) {
	var reported []string
	r, _ := newRack(t, Options{
		OnVisibility: func(key string, visible bool) {
			reported = append(reported, key)
		},
	}, "Osc", "Filter")

	r.SetHidden([]string{"osc", "filter"})
	filter := r.Module("filter")
	if got := filter.Get("style").Get("display").String(); got != "none" {
		t.Fatalf("setup: filter's display is %q, want none", got)
	}

	reported = nil
	r.SetHidden([]string{"osc"})

	if r.Hidden("filter") {
		t.Error("a key dropped from the set left its module hidden")
	}
	if got := filter.Get("style").Get("display").String(); got == "none" {
		t.Error("the module is no longer hidden but its element is still display:none")
	}
	if len(reported) != 0 {
		t.Errorf("a bulk restore called OnVisibility for %v", reported)
	}
}

func TestSetOrderRearranges(t *testing.T) {
	r, _ := newRack(t, Options{}, "Osc", "Filter", "VCA")
	r.SetOrder([]string{"vca", "osc", "filter"})
	if got := strings.Join(r.Order(), ","); got != "vca,osc,filter" {
		t.Errorf("Order = %q, want vca,osc,filter", got)
	}
}

// An order saved before a module existed still has to work, and the newcomer
// has to survive it — being dropped is the failure worth guarding.
func TestSetOrderKeepsModulesTheOrderDoesNotMention(t *testing.T) {
	r, _ := newRack(t, Options{}, "Osc", "Filter", "Newcomer")
	r.SetOrder([]string{"filter", "osc"})

	got := r.Order()
	if len(got) != 3 {
		t.Fatalf("Order = %v, a module was dropped", got)
	}
	if got[len(got)-2] != "filter" || got[len(got)-1] != "osc" {
		t.Errorf("Order = %v, want the named ones last in the order given", got)
	}
	if got[0] != "newcomer" {
		t.Errorf("Order = %v, want the unnamed module kept at the front", got)
	}
}

func TestSetOrderIgnoresKeysThatNameNoModule(t *testing.T) {
	r, _ := newRack(t, Options{}, "Osc", "Filter")
	r.SetOrder([]string{"nosuch", "filter", "alsonothere", "osc"})
	if got := strings.Join(r.Order(), ","); got != "filter,osc" {
		t.Errorf("Order = %q, want filter,osc", got)
	}
}

func TestOrderRoundTrips(t *testing.T) {
	r, _ := newRack(t, Options{}, "Osc", "Filter", "VCA")
	saved := r.Order()
	r.SetOrder([]string{"vca", "filter", "osc"})
	r.SetOrder(saved)
	if got := strings.Join(r.Order(), ","); got != strings.Join(saved, ",") {
		t.Errorf("Order = %q after restoring %v", got, saved)
	}
}

// Options fill in defaults so a zero Options is usable.
func TestNewFillsInDefaults(t *testing.T) {
	r, _ := newRack(t, Options{})
	if r.opts.ModuleClass != "rack-mod" || r.opts.HeaderClass != "rack-mod-hdr" {
		t.Errorf("class names default to %q/%q", r.opts.ModuleClass, r.opts.HeaderClass)
	}
	if r.opts.SlotWidth <= 0 || r.opts.Scale <= 0 || r.opts.Gap <= 0 {
		t.Errorf("a zero Options left SlotWidth=%v Scale=%v Gap=%v",
			r.opts.SlotWidth, r.opts.Scale, r.opts.Gap)
	}
}

// A negative gap or chrome is a mistake rather than a request; clamping to
// zero is what keeps the layout arithmetic sane.
func TestNewClampsNegativeMeasurements(t *testing.T) {
	r, _ := newRack(t, Options{Gap: -5, Chrome: -5})
	if r.opts.Gap != 0 || r.opts.Chrome != 0 {
		t.Errorf("negative measurements became Gap=%v Chrome=%v, want 0", r.opts.Gap, r.opts.Chrome)
	}
}

// The stylesheet goes in once however many racks are built, or every rack
// leaves another copy of it in the head.
// The stylesheet goes in once however many racks are built. Counted by its own
// id rather than by the size of the head, since a real page has other things
// in there.
func TestInjectCSSOnlyAddsTheStylesheetOnce(t *testing.T) {
	installFakeDOM()
	for i := 0; i < 5; i++ {
		InjectCSS()
	}
	head := document.Get("head")
	n := 0
	for i := 0; i < head.Get("children").Length(); i++ {
		if head.Get("children").Index(i).Get("id").String() == styleID {
			n++
		}
	}
	if n != 1 {
		t.Errorf("the head holds %d copies of the stylesheet after five InjectCSS calls, want 1", n)
	}
}

func TestCSSIsNotEmpty(t *testing.T) {
	if len(CSS()) == 0 {
		t.Error("the embedded stylesheet is empty")
	}
}
