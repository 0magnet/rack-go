//go:build js && wasm

package rack

import "syscall/js"

// A fake DOM, enough of one for this package.
//
// Node has no document, so the alternative to this is testing none of the code
// that touches one — which here is nearly all of it. It is built in JavaScript
// rather than assembled from Go because the shape is a tree of plain objects
// and saying so directly is shorter than a hundred js.FuncOf calls.
//
// Only what this package actually calls is implemented: class lists, a child
// list with appendChild's move semantics, a descendant query by class, and a
// style object. Anything else is left off deliberately, so a test that starts
// depending on more of the DOM fails here rather than passing by accident.
const fakeDOMSource = `
(function () {
  function El(tag) {
    return {
      tagName: (tag || "div").toUpperCase(),
      id: "",
      textContent: "",
      style: {
        _props: {},
        setProperty: function (k, v) { this._props[k] = v; },
        getPropertyValue: function (k) { return this._props[k] || ""; },
        removeProperty: function (k) { delete this._props[k]; },
      },
      _classes: [],
      children: [],
      parentNode: null,
      classList: {
        _el: null,
        add: function () {
          for (var i = 0; i < arguments.length; i++) {
            if (this._el._classes.indexOf(arguments[i]) < 0) this._el._classes.push(arguments[i]);
          }
        },
        remove: function () {
          for (var i = 0; i < arguments.length; i++) {
            var j = this._el._classes.indexOf(arguments[i]);
            if (j >= 0) this._el._classes.splice(j, 1);
          }
        },
        contains: function (c) { return this._el._classes.indexOf(c) >= 0; },
      },
      setAttribute: function () {},
      getBoundingClientRect: function () { return {left: 0, top: 0, right: 0, bottom: 0, width: 0, height: 0}; },
      addEventListener: function () {},
      removeEventListener: function () {},
      // appendChild moves rather than copies, which is the semantic SetOrder
      // depends on: appending a child already present takes it out of where it
      // was and puts it at the end.
      appendChild: function (c) {
        if (c.parentNode) {
          var k = c.parentNode.children.indexOf(c);
          if (k >= 0) c.parentNode.children.splice(k, 1);
        }
        c.parentNode = this;
        this.children.push(c);
        return c;
      },
      querySelector: function (sel) {
        var found = this.querySelectorAll(sel);
        return found.length ? found[0] : null;
      },
      querySelectorAll: function (sel) {
        var want = sel.charAt(0) === "." ? sel.slice(1) : null;
        var out = [];
        (function walk(node) {
          for (var i = 0; i < node.children.length; i++) {
            var c = node.children[i];
            if (want !== null ? c._classes.indexOf(want) >= 0 : c.tagName === sel.toUpperCase()) out.push(c);
            walk(c);
          }
        })(this);
        return out;
      },
    };
  }
  function make(tag) { var e = El(tag); e.classList._el = e; return e; }

  var doc = make("html");
  doc.head = make("head");
  doc.body = make("body");
  doc.createElement = function (tag) { return make(tag); };
  doc.getElementById = function (id) {
    var hit = null;
    (function walk(node) {
      for (var i = 0; i < node.children.length; i++) {
        if (node.children[i].id === id) hit = node.children[i];
        walk(node.children[i]);
      }
    })(doc.head);
    return hit;
  };
  var win = {
    getComputedStyle: function (el) {
      // Enough of a computed style for the visibility check: the inline
      // display if one was set, and the default otherwise.
      return {display: el.style.display || "block"};
    },
    addEventListener: function () {},
    removeEventListener: function () {},
    requestAnimationFrame: function (f) { f(0); return 0; },
    devicePixelRatio: 1,
  };
  globalThis.window = win;
  globalThis.document = doc;
  globalThis.__mkEl = make;
  return doc;
})()
`

// installFakeDOM makes sure there is a document to work against and returns a
// constructor for elements.
//
// Under a real browser — `go test -exec wasmbrowsertest` — the real one is
// used and the fake is not installed at all. window cannot be replaced there
// anyway, so the real getComputedStyle would be handed a plain object and
// refuse it. Running the same tests both ways is what keeps the fake honest:
// anything it gets wrong shows up as a test that passes under Node and fails
// in the browser.
func installFakeDOM() func(tag string) js.Value {
	if realDOM() {
		document = js.Global().Get("document")
		window = js.Global().Get("window")
		return func(tag string) js.Value {
			return document.Call("createElement", tag)
		}
	}
	js.Global().Call("eval", fakeDOMSource)
	document = js.Global().Get("document")
	window = js.Global().Get("window")
	return func(tag string) js.Value { return js.Global().Call("__mkEl", tag) }
}

// realDOM reports whether this is running in a browser rather than in Node.
func realDOM() bool {
	d := js.Global().Get("document")
	return d.Truthy() && d.Get("createElement").Type() == js.TypeFunction &&
		js.Global().Get("window").Truthy() &&
		js.Global().Get("window").Get("getComputedStyle").Type() == js.TypeFunction
}

// module builds a rack module element with a header carrying the given text,
// which is what Key reads to name it.
func module(mk func(string) js.Value, header string) js.Value {
	m := mk("div")
	m.Get("classList").Call("add", "rack-mod")
	h := mk("div")
	h.Get("classList").Call("add", "rack-mod-hdr")
	h.Set("textContent", header)
	m.Call("appendChild", h)
	return m
}
