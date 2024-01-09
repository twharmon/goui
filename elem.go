package goui

import (
	"reflect"
	"syscall/js"
)

// remove godom ... do it directly from here

// import { current, Ref } from './hooks.js';
// import { setAttr } from './reconcile.js';
type NoProps any

// type ComponentFn func(any) *Elem

type Children []*Elem

type Elem struct {
	tag    string
	ptr    uintptr
	render func() *Elem
	key    string
	props  any
	// attrs     Attributes
	ref       *Ref[js.Value]
	dom       js.Value
	unmounted bool

	virt        *Elem
	queue       []*Elem
	hooks       []any
	hooksCursor int
	memo        []any
}

func (e *Elem) Children() Children {
	return Children{e}
}

func (e *Elem) Attributes() *Attributes {
	a := e.props.(Attributes)
	return &a
}

func (e *Elem) teardown() {
	if e.ref != nil {
		e.ref.Value = js.Null()
	}
	if e.virt != nil {
		e.unmounted = true
		e.queue = nil
		for _, h := range e.hooks {
			if effect, ok := h.(*effectRecord); ok {
				if effect.teardown != nil {
					effect.teardown()
				}
			}
		}
		e.virt.teardown()
		return
	}
	if attrs, ok := e.props.(Attributes); ok {
		for _, ch := range attrs.Children {
			ch.teardown()
		}
	}
}

type Keyer interface {
	Key() string
}

type Memoer interface {
	Memo() Deps
}

func Component[T any](ty func(T) *Elem, props T) *Elem {
	fn := uintptr(reflect.ValueOf(ty).UnsafePointer())
	e := &Elem{
		ptr:    fn,
		render: func() *Elem { return ty(props) },
		props:  props,
	}
	if keyer, ok := any(props).(Keyer); ok {
		e.key = keyer.Key()
	}
	if memoer, ok := any(props).(Memoer); ok {
		e.memo = memoer.Memo()
	}
	return e
}

func Text(content string) *Elem {
	return &Elem{
		props: content,
	}
}

var currentElem *Elem

func callComponentFunc(elem *Elem) *Elem {
	prev := currentElem
	currentElem = elem
	elem.hooksCursor = 0
	vd := elem.render()
	if vd == nil {
		vd = &Elem{}
	}
	currentElem = prev
	return vd
}

type Callback[Func any] struct {
	Invoke Func
}

type Attributes struct {
	ID       string
	Class    string
	Disabled bool
	Style    string
	Value    string
	Children Children
	Key      string
	Type     string

	AriaHidden bool

	// Common UIEvents: https://developer.mozilla.org/en-US/docs/Web/API/UI_Events
	// All Events:      https://developer.mozilla.org/en-US/docs/Web/API/Event
	//
	// MouseEvent: click, dblclick, mouseup, mousedown
	// InputEvent: input, beforeinput
	// KeyboardEvent: keydown, keypress, keyup
	// CompositionEvent: compositionstart, compositionend, compositionupdate
	// WheelEvent: wheel
	// FocusEvent: focus, blur, focusin, and focusout

	OnClick     *Callback[func(*MouseEvent)]
	OnMouseMove *Callback[func(*MouseEvent)]
	OnInput     *Callback[func(*InputEvent)]
}

func Element(tag string, attrs Attributes) *Elem {
	elem := &Elem{tag: tag}
	elem.props = attrs
	elem.key = attrs.Key
	return elem
}

var namespacePrefix = "http://www.w3.org/"
var svgNamespace = namespacePrefix + "2000/svg"
var mathNamespace = namespacePrefix + "1998/Math/MathML"

func createDom(elem *Elem, ns string) js.Value {
	if elem.tag != "" {
		if elem.tag == "svg" {
			ns = svgNamespace
			elem.dom = createElementNS(elem.tag, ns)
		} else if elem.tag == "math" {
			ns = mathNamespace
			elem.dom = createElementNS(elem.tag, ns)
		} else {
			elem.dom = createElement(elem.tag)
		}
		if elem.ref != nil {
			elem.ref.Value = elem.dom
		}
		attrs := elem.props.(Attributes)
		if attrs.Disabled {
			elem.dom.Set("disabled", true)
		}
		if len(attrs.Class) > 0 {
			elem.dom.Set("className", attrs.Class)
		}
		if attrs.Style != "" {
			elem.dom.Set("style", attrs.Style)
		}
		if attrs.ID != "" {
			elem.dom.Set("id", attrs.ID)
		}
		if attrs.AriaHidden {
			elem.dom.Set("ariaHidden", true)
		}
		if attrs.Value != "" {
			elem.dom.Set("value", attrs.Value)
		}
		if attrs.OnClick != nil {
			elem.dom.Set("onclick", js.FuncOf(func(_ js.Value, args []js.Value) any {
				attrs.OnClick.Invoke(newMouseEvent(args[0]))
				return nil
			}))
		}
		for _, child := range attrs.Children {
			elem.dom.Call("appendChild", createDom(child, ns))
		}
	} else if elem.render != nil {
		elem.virt = callComponentFunc(elem)
		return createDom(elem.virt, ns)
	} else {
		elem.dom = createTextNode(elem.props.(string))
		return elem.dom
	}
	return elem.dom
}