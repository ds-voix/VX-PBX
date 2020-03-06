// https://www.kernel.org/doc/html/latest/input/event-codes.html
// https://www.kernel.org/doc/html/latest/input/uinput.html

// https://janczer.github.io/work-with-dev-input/
// https://godoc.org/github.com/gvalkov/golang-evdev#example-Open
// https://github.com/ds-voix/VX-PBX/blob/master/x%20switcher/draft.txt

// https://github.com/BurntSushi/xgb/blob/master/examples/get-active-window/main.go
package main

/*
 #cgo LDFLAGS: -lX11
 #include <X11/Xlib.h>
 #include <X11/XKBlib.h>
*/
import "C"

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/gvalkov/golang-evdev"
	"github.com/micmonay/keybd_event"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
)

//type uinput_user_dev C.struct_uinput_user_dev
//type timeval C.struct_timeval
//type input_event C.struct_input_event

type ctrl struct {
	KEY_LEFTCTRL bool;
	KEY_LEFTSHIFT bool;
	KEY_RIGHTSHIFT bool;
	KEY_LEFTALT bool;
	KEY_RIGHTCTRL bool;
	KEY_RIGHTALT bool;
	KEY_LEFTMETA bool;
	KEY_RIGHTMETA bool;
}

type t_key struct {
	code uint16;
	value int32; // 1=press 2=repeat 0=release
}

type t_keys []t_key

var (
	DEV_MOUCE = "/dev/input/event3"
	DEV_KEYBOARD = "/dev/input/event0"

	X *xgb.Conn
	kb keybd_event.KeyBonding
	window_keys = make(map [xproto.Window]t_keys) // keys pressed in window
	// !!! Note as windows are replaced in time, this structure will leak.
	// There must be added some TTL to just to remove "stolen cache" from map.
	window_last_seen = make(map [xproto.Window]time.Time)

	// const: each array is evaluated in go, so can't be declared as "const"
	LANG = t_keys{{evdev.KEY_LEFTCTRL, 1}, {evdev.KEY_LEFTCTRL, 0}} // Cyclic switch
	LANG_0 = t_keys{{evdev.KEY_LEFTSHIFT, 1}, {evdev.KEY_LEFTSHIFT, 0}} // Set lang #0 ("en" in my case)
	LANG_1 = t_keys{{evdev.KEY_RIGHTSHIFT, 1}, {evdev.KEY_RIGHTSHIFT, 0}} // Set lang #1 ("ru","by",etc)
	                                                                      // More cases? Sorry, not right now
	SWITCH = t_keys{{evdev.KEY_PAUSE, 1}, {evdev.KEY_PAUSE, 0}}
	SPACE = t_keys{{evdev.KEY_SPACE, 1}, {evdev.KEY_SPACE, 0}}

	keyboardEvents = make(chan t_key, 4)
	miceEvents = make(chan t_key, 4)
)

// There must be 1 buffer per each X-window.
// Or just reset the buffer on each focus change?
func ActiveWindowId() (xproto.Window) { // xproto.Window == uint32
	// Get the window id of the root window.
	setup := xproto.Setup(X)
	root := setup.DefaultScreen(X).Root

	// Get the atom id (i.e., intern an atom) of "_NET_ACTIVE_WINDOW".
	aname := "_NET_ACTIVE_WINDOW"
	activeAtom, err := xproto.InternAtom(X, true, uint16(len(aname)),
		aname).Reply()
	if err != nil {
		return 0
	}

	// Get the actual value of _NET_ACTIVE_WINDOW.
	// Note that 'reply.Value' is just a slice of bytes, so we use an
	// XGB helper function, 'Get32', to pull an unsigned 32-bit integer out
	// of the byte slice. We then convert it to an X resource id so it can
	// be used to get the name of the window in the next GetProperty request.
	reply, err := xproto.GetProperty(X, false, root, activeAtom.Atom,
		xproto.GetPropertyTypeAny, 0, (1<<32)-1).Reply()
	if err != nil {
		return 0
	}
	windowId := xproto.Window(xgb.Get32(reply.Value))
	return windowId
//	fmt.Printf("Active window id: %X\n", windowId)
}


func Compare(pattern t_keys, back int) (bool) {
    if len(window_keys[ActiveWindowId()]) - back < len(pattern) {
		return false
	}
	l := len(pattern)
	offset := len(window_keys[ActiveWindowId()]) - l - back
	for i := l - 1; i >= 0; i-- {
		if pattern[i] != window_keys[ActiveWindowId()][offset + i] {
			return false
		}
	}
	return true
}


func CtrlSeqence() (bool) { // CTRL + some_key
	var ctrl = t_key{evdev.KEY_LEFTCTRL, 0}
	l := len(window_keys[ActiveWindowId()])
	w := ActiveWindowId()
	if l < 4 { return false	}

	if window_keys[w][l - 1] != ctrl {
		return false
	}
	if window_keys[w][l - 2].code != evdev.KEY_LEFTCTRL {
		return true
	}
	return false
}


func SpaceSeqence() (bool) { // some_key after space
	var space = t_key{evdev.KEY_SPACE, 0}
	l := len(window_keys[ActiveWindowId()])
	w := ActiveWindowId()
	if l < 4 { return false	}

	if window_keys[w][l - 1].code == evdev.KEY_SPACE ||  window_keys[w][l - 1].code == evdev.KEY_BACKSPACE {
		return false
	}
	if window_keys[w][l - 2].code == evdev.KEY_SPACE ||  window_keys[w][l - 2].code == evdev.KEY_BACKSPACE {
		return false
	}
	if window_keys[w][l - 3] != space {
		return false
	}
	if window_keys[w][l - 4].code != evdev.KEY_SPACE {
		return false
	}
	return true
}


func Drop() {
	window_keys[ActiveWindowId()] = nil
	return
}


func Add(event t_key) {
	code := uint16(event.code)
	value := int32(event.value)

	window_keys[ActiveWindowId()] = append(window_keys[ActiveWindowId()], t_key{code, value})
	l := len(window_keys[ActiveWindowId()])

	if Compare(SWITCH, 0) {
		Switch(window_keys[ActiveWindowId()][ : l-len(SWITCH)])
		return
	}
	if Compare(LANG_0, 0) {
		LanguageSwitch(0)
		window_keys[ActiveWindowId()] = nil
		return
	}
	if Compare(LANG_1, 0) {
		LanguageSwitch(1)
		window_keys[ActiveWindowId()] = nil
		return
	}
	if Compare(LANG, 0) {
		LanguageSwitch(-1)
		window_keys[ActiveWindowId()] = nil
		return
	}

	if CtrlSeqence() {
		window_keys[ActiveWindowId()] = nil
		return
	}

	if SpaceSeqence() {
	    fmt.Printf("%d %v", code, window_keys[ActiveWindowId()])
		window_keys[ActiveWindowId()] = t_keys{ window_keys[ActiveWindowId()][l - 2], window_keys[ActiveWindowId()][l - 1] }
	}

	return
}


func LanguageSwitch(lang int) {
	state := new(_Ctype_struct__XkbStateRec)
	layout := _Ctype_uint(0)

	display := C.XOpenDisplay(nil);
    if display == nil {
		fmt.Printf("Errot while XOpenDisplay()! Active window id: %X\n", ActiveWindowId())
		return
    }

	C.XkbGetState(display, C.XkbUseCoreKbd, state);
	if lang < 0 {
		if state.group > 0 {
			layout = 0
		} else {
			layout = 1
		}
	} else {
		layout = _Ctype_uint(lang)
	}

    C.XkbLockGroup(display, C.XkbUseCoreKbd, layout);
	C.XkbGetState(display, C.XkbUseCoreKbd, state);
//    fmt.Println(state.group)
//    time.Sleep(100 * time.Millisecond) // In KDE, language swtching through such a trick takes more than 300ms!
}

func Switch (keys t_keys) {
    // Reset window_keys: I daresay that there's no need to remember all shit
    window_keys = make(map [xproto.Window]t_keys)
//	fmt.Printf("Active window id: %X %v\n", display, keys)

	bs_count := 0
	char_count := 0
	caps_lock := false
	num_lock := false

	for i := 0; i < len(keys); i++ {
		key := keys[i].code
		val := keys[i].value
		switch key {
		case evdev.KEY_LEFTCTRL:
		case evdev.KEY_LEFTSHIFT:
		case evdev.KEY_RIGHTSHIFT:
		case evdev.KEY_LEFTALT:
		case evdev.KEY_RIGHTCTRL:
		case evdev.KEY_RIGHTALT:
		case evdev.KEY_LEFTMETA:
		case evdev.KEY_RIGHTMETA:
		case evdev.KEY_CAPSLOCK:
			if val == 0 { caps_lock = !caps_lock }
		case evdev.KEY_NUMLOCK:
			if val == 0 { num_lock = !num_lock }
		case evdev.KEY_BACKSPACE:
			if val > 0 { bs_count++ }

		default:
			if val > 0 { char_count++ }
		}
	}

	for i := bs_count; i < char_count; i++ {
		kb.SetKeys(evdev.KEY_BACKSPACE)
		err := kb.Launching()
		if err != nil {
			panic(err)
		}
		kb.Clear()
	}

	if caps_lock { // invert CAPS_LOCK before replay
		kb.SetKeys(evdev.KEY_CAPSLOCK)
		err := kb.Launching()
		if err != nil {
			panic(err)
		}
		kb.Clear()
	}
	if num_lock { // invert NUM_LOCK before replay
		kb.SetKeys(evdev.KEY_NUMLOCK)
		err := kb.Launching()
		if err != nil {
			panic(err)
		}
		kb.Clear()
	}

 	LanguageSwitch(-1)

	if bs_count >= char_count {
		return
	}

	for i := 0; i < len(keys); i++ {
		key := keys[i].code
		val := keys[i].value
//        Add(key, val)
		window_keys[ActiveWindowId()] = append(window_keys[ActiveWindowId()], t_key{key, val})

		switch key {
		case evdev.KEY_LEFTCTRL:
			kb.HasCTRL(val > 0)
		case evdev.KEY_LEFTSHIFT:
			kb.HasSHIFT(val > 0)
		case evdev.KEY_RIGHTSHIFT: // CTRL
			kb.HasSHIFTR(val > 0)
		case evdev.KEY_LEFTALT:
			kb.HasALT(val > 0)
		case evdev.KEY_RIGHTCTRL:
			kb.HasCTRLR(val > 0)
		case evdev.KEY_RIGHTALT:
			kb.HasALTGR(val > 0)
		case evdev.KEY_LEFTMETA:
			kb.HasSuper(val > 0)
		case evdev.KEY_RIGHTMETA:
			kb.HasSuper(val > 0)
		default:
		    if val > 0 {
			kb.SetKeys(int(key))
			err := kb.Launching()
			  if err != nil {
			  	panic(err)
			  }
			}
		}
	}
    kb.Clear()
}


func mouce() {
	device, _ := evdev.Open(DEV_MOUCE)
	fmt.Println(device)

	for {
		event, _ := device.ReadOne()
		if event.Type == evdev.EV_MSC { // Button events
			miceEvents <- t_key{event.Code, event.Value}
		}
	}
}


func keyboard() {
	device, _ := evdev.Open(DEV_KEYBOARD)
	fmt.Println(device)

	for {
		event, _ := device.ReadOne()
		if event.Type == evdev.EV_KEY { // Key events
			keyboardEvents <- t_key{event.Code, event.Value}
		}
	}
}


func main() {
	var err error

	X, err = xgb.NewConn()
	if err != nil {
		panic(err)
	}

	kb, err = keybd_event.NewKeyBonding()
	if err != nil {
		panic(err)
	}

    go mouce()
    go keyboard()

    var event t_key
	for {
		select {
		case event = <- miceEvents: // code is always 0x4, while value is 0x90000 + button(1,2,3...)
				Drop()
				continue
		case event = <- keyboardEvents:
		}

		switch key := event.code; {
			case key == evdev.KEY_BREAK || key == evdev.KEY_PAUSE:
				Add(event)
			case key < evdev.KEY_1: // drop
				Drop()
			case key == evdev.KEY_MINUS: // drop
				Drop()
			case key < evdev.KEY_BACKSPACE: // pass
				Add(event)
			case key == evdev.KEY_BACKSPACE: // pass !!! but don't count as char
				Add(event)
			case key < evdev.KEY_Q: // drop
				Drop()
			case key < evdev.KEY_ENTER: // pass
				Add(event)
			case key < evdev.KEY_LEFTCTRL: // drop
				Drop()
			case key == evdev.KEY_LEFTCTRL: // CTRL
				Add(event)
			case key <= evdev.KEY_LEFTSHIFT: // pass
				Add(event)
			case key <= evdev.KEY_RIGHTSHIFT: // pass
				Add(event)
			case key == evdev.KEY_KPASTERISK: // pass keypad
				Add(event)
			case key == evdev.KEY_LEFTALT: // CTRL
				Add(event)
			case key == evdev.KEY_SPACE: // pass
				Add(event)
			case key == evdev.KEY_CAPSLOCK: // pass
				Add(event)
			case key <= evdev.KEY_F10: // F1..F10 ignore
			case key == evdev.KEY_F11: // F11 ignore
			case key == evdev.KEY_F12: // F12 ignore
			case key <= evdev.KEY_SCROLLLOCK: // pass
				Add(event)
			case key < evdev.KEY_ZENKAKUHANKAKU: // pass keypad
				Add(event)
			case key == evdev.KEY_KPCOMMA: // pass keypad
				Add(event)
			case key == evdev.KEY_KPLEFTPAREN: // pass keypad
				Add(event)
			case key == evdev.KEY_KPRIGHTPAREN: // pass keypad
				Add(event)
			case key == evdev.KEY_RIGHTCTRL: // CTRL
				Add(event)
			case key == evdev.KEY_KPSLASH: // pass
				Add(event)
			case key == evdev.KEY_RIGHTALT: // CTRL
				Add(event)
			case key == evdev.KEY_LEFTMETA: // ???
				Add(event)
			case key == evdev.KEY_RIGHTMETA: // ???
				Add(event)
			default: // drop
				Drop()
		}
	}

//	event, _ := device.ReadOne()
	uinput, err := os.OpenFile("/dev/uinput", os.O_WRONLY | syscall.O_NONBLOCK, 0600)
//    time.Sleep(2 * time.Second)
    bs := make([]byte, 24)
    binary.LittleEndian.PutUint16(bs[16:], evdev.EV_KEY) // Type
    binary.LittleEndian.PutUint16(bs[18:], 46) // Code
    binary.LittleEndian.PutUint32(bs[20:], 1) // Value = key_press
//    binary.Write(uinput, binary.LittleEndian, &ev)
    uinput.Write(bs)
    fmt.Printf("xxx %x\n", bs)

    bs = make([]byte, 24)
    binary.LittleEndian.PutUint16(bs[16:], evdev.EV_KEY) // Type
    binary.LittleEndian.PutUint16(bs[18:], 46) // Code
    binary.LittleEndian.PutUint32(bs[20:], 0) // Value = key_release
    fmt.Printf("yyy %x\n", bs)
    uinput.Write(bs)


    os.Exit(0)
//	f, err := os.Open("/dev/input/mouse0")
	f, err := os.Open("/dev/input/event0")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	b := make([]byte, 24)
	for {
		f.Read(b)
		sec := binary.LittleEndian.Uint64(b[0:8])
		usec := binary.LittleEndian.Uint64(b[8:16])
		t := time.Unix(int64(sec), int64(usec))
		fmt.Println(t)
		var value int32
		typ := binary.LittleEndian.Uint16(b[16:18])
		code := binary.LittleEndian.Uint16(b[18:20])
		binary.Read(bytes.NewReader(b[20:]), binary.LittleEndian, &value)
		fmt.Printf("type: %x\ncode: %d\nvalue: %d\n", typ, code, value)
	}
}
