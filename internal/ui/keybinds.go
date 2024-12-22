package ui

import (
	"log/slog"
	"slices"
	"strings"

	"github.com/abenz1267/walker/internal/modules"
	"github.com/abenz1267/walker/internal/modules/clipboard"
	"github.com/abenz1267/walker/internal/util"
	"github.com/diamondburned/gotk4/pkg/core/gioutil"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
)

type keybinds map[int]map[gdk.ModifierType]func() bool

var (
	binds   keybinds
	aibinds keybinds
)

var (
	modifiersInt = map[string]int{
		"lctrl":     gdk.KEY_Control_L,
		"rctrl":     gdk.KEY_Control_R,
		"lalt":      gdk.KEY_Alt_L,
		"ralt":      gdk.KEY_Alt_R,
		"lshift":    gdk.KEY_Shift_L,
		"rshift":    gdk.KEY_Shift_R,
		"shiftlock": gdk.KEY_Shift_Lock,
	}
	modifiers = map[string]gdk.ModifierType{
		"ctrl":   gdk.ControlMask,
		"lctrl":  gdk.ControlMask,
		"rctrl":  gdk.ControlMask,
		"alt":    gdk.AltMask,
		"lalt":   gdk.AltMask,
		"ralt":   gdk.AltMask,
		"lshift": gdk.ShiftMask,
		"rshift": gdk.ShiftMask,
		"shift":  gdk.ShiftMask,
	}
	specialKeys = map[string]int{
		"backspace": int(gdk.KEY_BackSpace),
		"tab":       int(gdk.KEY_Tab),
		"esc":       int(gdk.KEY_Escape),
		"enter":     int(gdk.KEY_Return),
		"down":      int(gdk.KEY_Down),
		"up":        int(gdk.KEY_Up),
		"stab":      int(gdk.KEY_ISO_Left_Tab),
	}
	labelTrigger        = gdk.KEY_Alt_L
	keepOpenModifier    = gdk.ShiftMask
	labelModifier       = gdk.AltMask
	activateAltModifier = gdk.AltMask
)

func parseKeybinds() {
	binds = make(keybinds)
	aibinds = make(keybinds)

	binds.validate(cfg.Keys.AcceptTypeahead)
	binds.bind(binds, cfg.Keys.AcceptTypeahead, acceptTypeahead)

	binds.validate(cfg.Keys.Close)
	binds.bind(binds, cfg.Keys.Close, quitKeybind)

	binds.validate(cfg.Keys.Next)
	binds.bind(binds, cfg.Keys.Next, selectNext)

	binds.validate(cfg.Keys.Prev)
	binds.bind(binds, cfg.Keys.Prev, selectPrev)

	binds.validate(cfg.Keys.RemoveFromHistory)
	binds.bind(binds, cfg.Keys.RemoveFromHistory, deleteFromHistory)

	binds.validate(cfg.Keys.ResumeQuery)
	binds.bind(binds, cfg.Keys.ResumeQuery, resume)

	binds.validate(cfg.Keys.ToggleExactSearch)
	binds.bind(binds, cfg.Keys.ToggleExactSearch, toggleExactMatch)

	binds.bind(binds, "enter", func() bool { return activate(false, false) })
	binds.bind(binds, strings.Join([]string{cfg.Keys.ActivationModifiers.KeepOpen, "enter"}, " "), func() bool { return activate(true, false) })
	binds.bind(binds, strings.Join([]string{cfg.Keys.ActivationModifiers.Alternate, "enter"}, " "), func() bool { return activate(false, true) })

	keepOpenModifier = modifiers[cfg.Keys.ActivationModifiers.KeepOpen]
	activateAltModifier = modifiers[cfg.Keys.ActivationModifiers.Alternate]

	binds.validateTriggerLabels(cfg.Keys.TriggerLabels)
	labelTrigger = modifiersInt[strings.Fields(cfg.Keys.TriggerLabels)[0]]
	labelModifier = modifiers[strings.Fields(cfg.Keys.TriggerLabels)[0]]

	binds.validate(cfg.Keys.Ai.ClearSession)
	binds.bind(aibinds, cfg.Keys.Ai.ClearSession, aiClearSession)

	binds.validate(cfg.Keys.Ai.CopyLastResponse)
	binds.bind(aibinds, cfg.Keys.Ai.CopyLastResponse, aiCopyLast)

	binds.validate(cfg.Keys.Ai.ResumeSession)
	binds.bind(aibinds, cfg.Keys.Ai.ResumeSession, aiResume)

	binds.validate(cfg.Keys.Ai.RunLastResponse)
	binds.bind(aibinds, cfg.Keys.Ai.RunLastResponse, aiExecuteLast)
}

func (keybinds) bind(binds keybinds, val string, fn func() bool) {
	fields := strings.Fields(val)

	m := []gdk.ModifierType{}

	key := 0

	for _, v := range fields {
		if len(v) > 1 {
			if val, exists := modifiers[v]; exists {
				m = append(m, val)
			}

			if val, exists := specialKeys[v]; exists {
				key = val
			}
		} else {
			key = int(v[0])
		}
	}

	modifier := gdk.NoModifierMask

	switch len(m) {
	case 1:
		modifier = m[0]
	case 2:
		modifier = m[0] | m[1]
	case 3:
		modifier = m[0] | m[1] | m[2]
	}

	_, ok := binds[key]
	if !ok {
		binds[key] = make(map[gdk.ModifierType]func() bool)
	}

	binds[key][modifier] = fn
}

func (keybinds) execute(key int, modifier gdk.ModifierType) bool {
	if isAi {
		fn, ok := aibinds[key][modifier]
		if ok {
			return fn()
		}
	}

	if fn, ok := binds[key][modifier]; ok {
		return fn()
	}

	return false
}

func (keybinds) validate(bind string) {
	fields := strings.Fields(bind)

	for _, v := range fields {
		if len(v) > 1 {
			_, existsMod := modifiers[v]
			_, existsSpecial := specialKeys[v]

			if !existsMod && !existsSpecial {
				slog.Error("invalid keybind", bind, "key", v)
			}
		}
	}
}

func (keybinds) validateTriggerLabels(bind string) {
	fields := strings.Fields(bind)
	_, exists := modifiersInt[fields[0]]

	if !exists || len(fields[0]) == 1 {
		slog.Error("invalid trigger_label keybind", bind)
	}
}

func toggleAM() bool {
	if cfg.ActivationMode.Disabled {
		return false
	}

	if common.selection.NItems() != 0 {
		enableAM()

		return true
	}

	return false
}

func deleteFromHistory() bool {
	if singleModule != nil && singleModule.General().Name == cfg.Builtins.Clipboard.Name {
		entry := gioutil.ObjectValue[util.Entry](common.items.Item(common.selection.Selected()))
		singleModule.(*clipboard.Clipboard).Delete(entry)
		debouncedProcess(process)
		return true
	}

	entry := gioutil.ObjectValue[util.Entry](common.items.Item(common.selection.Selected()))
	hstry.Delete(entry.Identifier())

	return true
}

func aiCopyLast() bool {
	if !isAi {
		return false
	}

	ai := findModule(cfg.Builtins.AI.Name, toUse, explicits).(*modules.AI)
	ai.CopyLastResponse()

	return true
}

func aiExecuteLast() bool {
	if !isAi {
		return false
	}

	ai := findModule(cfg.Builtins.AI.Name, toUse, explicits).(*modules.AI)
	ai.RunLastMessageInTerminal()
	quit(true)

	return true
}

func toggleExactMatch() bool {
	text := elements.input.Text()

	if strings.HasPrefix(text, "'") {
		elements.input.SetText(strings.TrimPrefix(text, "'"))
	} else {
		elements.input.SetText("'" + text)
	}

	elements.input.SetPosition(-1)

	return true
}

func resume() bool {
	if appstate.LastQuery != "" {
		elements.input.SetText(appstate.LastQuery)
		elements.input.SetPosition(-1)
		elements.input.GrabFocus()
	}

	return true
}

func aiResume() bool {
	if !isAi {
		return false
	}

	ai := findModule(cfg.Builtins.AI.Name, toUse, explicits).(*modules.AI)
	ai.ResumeLastMessages()

	return true
}

func aiClearSession() bool {
	if !isAi {
		return false
	}

	ai := findModule(cfg.Builtins.AI.Name, toUse, explicits).(*modules.AI)
	elements.input.SetText("")
	ai.ClearCurrent()

	return true
}

func activateFunctionKeys(val uint) bool {
	index := slices.Index(fkeys, val)

	if index != -1 {
		selectActivationMode(false, true, uint(index))
		return true
	}

	return false
}

func activateKeepOpenFunctionKeys(val uint) bool {
	index := slices.Index(fkeys, val)

	if index != -1 {
		selectActivationMode(true, true, uint(index))
		return true
	}

	return false
}

func quitKeybind() bool {
	if appstate.IsDmenu {
		handleDmenuResult("CNCLD")
	}

	if cfg.IsService {
		quit(false)
		return true
	} else {
		exit(false, true)
		return true
	}
}

func acceptTypeahead() bool {
	if elements.typeahead.Text() != "" {
		tahAcceptedIdentifier = tahSuggestionIdentifier
		tahSuggestionIdentifier = ""

		elements.input.SetText(elements.typeahead.Text())
		elements.input.SetPosition(-1)

		return true
	}

	return false
}

func activate(keepOpen bool, isAlt bool) bool {
	if appstate.ForcePrint && elements.grid.Model().NItems() == 0 {
		if appstate.IsDmenu {
			handleDmenuResult(elements.input.Text())
		}

		closeAfterActivation(keepOpen, false)
		return true
	}

	activateItem(keepOpen, isAlt)
	return true
}
