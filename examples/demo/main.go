//go:build js && wasm

// Demo for rack-go: a rack of modules with varying content, laid out on the
// slot pitch, with switches to put modules away and header-dragging to
// rearrange them.
package main

import (
	"fmt"
	"syscall/js"

	rack "github.com/0magnet/rack-go"
)

var document = js.Global().Get("document")

// div makes a classed div, which is every element this demo builds.
func div(class string) js.Value {
	e := document.Call("createElement", "div")
	e.Set("className", class)
	return e
}

func onClick(id string, fn func()) {
	btn := document.Call("getElementById", id)
	if !btn.Truthy() {
		return
	}
	btn.Set("onclick", js.FuncOf(func(js.Value, []js.Value) interface{} {
		fn()
		return nil
	}))
}

// module builds one module: a header and a grid of n cells. The cells are what
// the quantizer measures — the grid fills columns top to bottom and wraps, so
// a module with more cells needs more columns and is snapped to more slots.
func module(title string, cells int) js.Value {
	m := div("rack-mod demo-mod")
	h := div("rack-mod-hdr")
	h.Set("textContent", title)
	m.Call("appendChild", h)

	g := div("rack-grid")
	for i := 0; i < cells; i++ {
		c := div("demo-cell")
		c.Set("textContent", fmt.Sprintf("%s %d", title[:1], i+1))
		g.Call("appendChild", c)
	}
	m.Call("appendChild", g)
	return m
}

func main() {
	host := document.Call("getElementById", "rack-host")

	r := rack.New(rack.Options{
		Container: host,
		// The Console holds the switches, so it is the one module that cannot
		// be put away — otherwise the rack can be emptied with no way back.
		Pinned: func(key string) bool { return key == "console" },
		OnReorder: func(order []string) {
			fmt.Println("order:", order)
		},
		OnVisibility: func(key string, shown bool) {
			fmt.Println("visibility:", key, shown)
		},
	})

	// A console, then modules of deliberately different content widths so the
	// quantization is visible: 3 cells fits one slot, 24 needs four.
	console := div("rack-mod demo-mod")
	ch := div("rack-mod-hdr")
	ch.Set("textContent", "Console")
	console.Call("appendChild", ch)
	switches := div("rack-grid")
	console.Call("appendChild", switches)
	r.Add(console)

	for _, m := range []struct {
		title string
		cells int
	}{
		{"Oscillator", 12},
		{"Filter", 6},
		{"Envelope", 3},
		{"Mixer", 24},
		{"Reverb", 8},
		{"Output", 3},
	} {
		r.Add(module(m.title, m.cells))
	}

	r.Switches(switches)
	r.Quantize()

	onClick("btn-bigger", func() { r.SetScale(r.Scale() + 0.2) })
	onClick("btn-smaller", func() { r.SetScale(r.Scale() - 0.2) })
	onClick("btn-column", func() {
		host.Get("classList").Call("toggle", "rack-column")
		r.Quantize()
	})
	onClick("btn-report", func() {
		fmt.Println("order:", r.Order(), "hidden:", r.HiddenKeys(), "scale:", r.Scale())
	})

	select {}
}
