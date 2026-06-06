package ui

import (
	"context"
	"image"
	"image/color"
	"log/slog"
	"os"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/InfiniteSkye/electrorangerd/internal/adapter/ui/panel"
	"github.com/InfiniteSkye/electrorangerd/internal/port"
)

// App is the root of the GIOUI adapter. It owns the four-mode state machine,
// the master-password gate, and the peek panel. Per-mode views are lazily
// constructed after the vault unlocks.
type App struct {
	// Service ports.
	forward    port.ForwardEngineer
	reverse    port.ReverseEngineer
	drift      port.DriftDetector
	project    port.ProjectService
	validator  port.Validator
	dictionary port.DictionaryService
	vault      port.Vault

	// Per-mode History stacks. Forward and reverse engineering do not have
	// undo/redo — they are one-shot operations, not edit sessions.
	diagramHistory    port.History
	dictionaryHistory port.History

	// Infrastructure.
	log   *slog.Logger
	theme *Theme

	// Password gate.
	password *passwordView

	// Mode state.
	mode       Mode
	modeClicks [len(allModes)]widget.Clickable
	peekToggle widget.Clickable

	// Per-mode views (built after the password gate clears).
	diagramView    *diagramView
	forwardView    *forwardView
	dictionaryView *dictionaryView
	reverseView    *reverseView

	// Peek panel — same UI element regardless of which mode is active.
	peek *panel.Peek
}

// NewApp constructs the App with all required port dependencies and a freshly
// created theme. It does not open a window; call Run to start the event loop.
func NewApp(
	fwd port.ForwardEngineer,
	rev port.ReverseEngineer,
	drift port.DriftDetector,
	proj port.ProjectService,
	val port.Validator,
	diagramHist port.History,
	dictionaryHist port.History,
	dict port.DictionaryService,
	vault port.Vault,
	log *slog.Logger,
) *App {
	a := &App{
		forward:           fwd,
		reverse:           rev,
		drift:             drift,
		project:           proj,
		validator:         val,
		dictionary:        dict,
		vault:             vault,
		diagramHistory:    diagramHist,
		dictionaryHistory: dictionaryHist,
		log:               log,
		theme:             NewTheme(),
		mode:              ModeDiagram,
	}
	a.peek = panel.New(dict)
	return a
}

// Run launches the Gio event loop. The window goroutine is started first,
// then app.Main() is called on the calling goroutine (which must be the OS
// main thread on macOS). Run only returns if the window loop exits with an
// error before os.Exit is called.
func (a *App) Run() error {
	go func() {
		w := new(app.Window)
		w.Option(
			app.Title("ElectroRangerD"),
			app.Size(unit.Dp(1024), unit.Dp(768)),
		)
		if err := a.loop(w); err != nil {
			a.log.Error("ui loop", "err", err)
		}
		os.Exit(0)
	}()
	app.Main()
	return nil
}

// loop is the per-window event loop. It runs on the window goroutine until
// the window is closed or an unrecoverable error occurs.
func (a *App) loop(w *app.Window) error {
	var ops op.Ops
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			a.frame(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

// frame is called once per FrameEvent. It fills the background, gates on the
// master-password prompt, and (once unlocked) dispatches to the active mode
// view and the peek panel.
func (a *App) frame(gtx layout.Context) {
	a.fillBackground(gtx, color.NRGBA{R: 0x0A, G: 0x0B, B: 0x10, A: 0xFF})

	if a.password == nil {
		a.password = newPasswordView(a.vault, a.log)
		a.password.tryKeychainUnlock(context.Background())
	}
	if !a.password.Done() {
		a.password.Layout(gtx, a.theme.Material)
		return
	}
	if a.diagramView == nil {
		a.initViews()
	}

	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.layoutModeNav(gtx)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.layoutCurrentView(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.peek.Layout(gtx, a.theme.Material)
				}),
			)
		}),
	)
}

// initViews creates the per-mode views once the master vault is unlocked.
// They are not constructed eagerly because the dictionary view's markdown
// renderer touches the theme and we want zero work before authentication.
func (a *App) initViews() {
	a.diagramView = newDiagramView(a.diagramHistory)
	a.forwardView = newForwardView(a.forward)
	a.dictionaryView = newDictionaryView(a.dictionary, a.dictionaryHistory, a.theme.Material)
	a.reverseView = newReverseView(a.reverse)
}

func (a *App) fillBackground(gtx layout.Context, c color.NRGBA) {
	rect := clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, gtx.Constraints.Max.Y)}.Push(gtx.Ops)
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	rect.Pop()
}

// layoutModeNav draws the top navigation bar with one button per mode and a
// peek-toggle button at the right.
func (a *App) layoutModeNav(gtx layout.Context) layout.Dimensions {
	return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, len(allModes)*2+2)
		for i, m := range allModes {
			i, m := i, m
			if a.modeClicks[i].Clicked(gtx) {
				a.mode = m
			}
			label := m.Label()
			if a.mode == m {
				label = "▸ " + label
			}
			children = append(children,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Button(a.theme.Material, &a.modeClicks[i], label).Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
			)
		}
		children = append(children,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: gtx.Constraints.Min}
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if a.peekToggle.Clicked(gtx) {
					a.peek.Open = !a.peek.Open
				}
				label := "Peek ▸"
				if a.peek.Open {
					label = "Peek ▾"
				}
				return material.Button(a.theme.Material, &a.peekToggle, label).Layout(gtx)
			}),
		)
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

func (a *App) layoutCurrentView(gtx layout.Context) layout.Dimensions {
	th := a.theme.Material
	switch a.mode {
	case ModeDiagram:
		return a.diagramView.Layout(gtx, th)
	case ModeForward:
		return a.forwardView.Layout(gtx, th)
	case ModeDictionary:
		return a.dictionaryView.Layout(gtx, th)
	case ModeReverse:
		return a.reverseView.Layout(gtx, th)
	}
	return layout.Dimensions{}
}
