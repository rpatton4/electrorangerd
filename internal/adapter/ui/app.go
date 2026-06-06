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

	"github.com/InfiniteSkye/electrorangerd/internal/adapter/ui/panel"
	"github.com/InfiniteSkye/electrorangerd/internal/adapter/ui/theme"
	"github.com/InfiniteSkye/electrorangerd/internal/port"
)

// screenType distinguishes the top-level UI surfaces inside the app shell.
// The master-password gate runs unconditionally before either screen — it
// is a precondition, not a screen state. After unlock the welcome chooser
// is the first surface; picking a mode advances to the mode shell.
type screenType int

const (
	screenWelcome screenType = iota
	screenMode
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
	theme *theme.Theme

	// Password gate.
	password *passwordView

	// Screen state — welcome chooser vs. active-mode shell.
	screen screenType

	// Mode state.
	mode Mode

	// Nav state — single circular button at the mode-shell bottom. Click
	// toggles the menu overlay (the larger chrome dial with the four mode
	// labels) which floats centred on top of the mode shell.
	navClick   widget.Clickable
	navButton  paint.ImageOp
	menuOpen   bool
	menuClick  widget.Clickable
	menuButton paint.ImageOp

	// Per-mode views (built after the password gate clears).
	welcomeView    *welcomeView
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
	th *theme.Theme,
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
		theme:             th,
		mode:              ModeDiagram,
		screen:            screenWelcome,
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

// frame is called once per FrameEvent. It fills the background, ensures
// the views are constructed on the first frame, gates unconditionally on
// the master-password prompt, and (once unlocked) dispatches to the
// current screen (welcome chooser or mode shell).
func (a *App) frame(gtx layout.Context) {
	a.fillBackground(gtx, a.theme.Surface)

	if a.password == nil {
		a.initViews()
		a.password.tryKeychainUnlock(context.Background())
	}

	if !a.password.Done() {
		a.password.Layout(gtx, a.theme)
		return
	}

	switch a.screen {
	case screenWelcome:
		a.welcomeView.Layout(gtx, a.theme)
	case screenMode:
		layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return a.layoutCurrentView(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.peek.Layout(gtx, a.theme)
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.layoutModeNav(gtx)
			}),
		)
		if a.menuOpen {
			a.layoutMenuOverlay(gtx)
		}
	}
}

// initViews builds the password gate, the welcome chooser, and the four
// per-mode views. Called once at the start of the first frame — the nil
// a.password serves as the "first frame" sentinel.
func (a *App) initViews() {
	a.password = newPasswordView(a.vault, a.log)
	a.navButton = decodeNavButton(a.log)
	a.menuButton = decodeMenuButton(a.log)
	a.welcomeView = newWelcomeView(func(m Mode) {
		a.mode = m
		a.screen = screenMode
	})
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

// layoutModeNav draws the bottom navigation bar — a single circular button
// rendered from the embedded chrome PNG, centred horizontally. Clicking
// it toggles the menu overlay (see layoutMenuOverlay).
func (a *App) layoutModeNav(gtx layout.Context) layout.Dimensions {
	if a.navClick.Clicked(gtx) {
		a.menuOpen = !a.menuOpen
	}

	return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return a.navClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				sz := gtx.Dp(unit.Dp(56))
				size := image.Pt(sz, sz)
				gtx.Constraints = layout.Exact(size)

				defer clip.Ellipse{Max: size}.Push(gtx.Ops).Pop()
				return widget.Image{
					Src:      a.navButton,
					Fit:      widget.Cover,
					Position: layout.Center,
				}.Layout(gtx)
			})
		})
	})
}

// layoutMenuOverlay renders the larger chrome dial (with the four mode
// labels around the rim) floating centred on top of the mode shell. The
// dial's transparency comes from the decoder's inscribed-circle mask —
// no render-time clip needed. The click is registered but currently does
// nothing; tap the bottom nav button again to dismiss.
func (a *App) layoutMenuOverlay(gtx layout.Context) layout.Dimensions {
	_ = a.menuClick.Clicked(gtx) // consume; menu does nothing yet

	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.menuClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			sz := gtx.Dp(unit.Dp(320))
			size := image.Pt(sz, sz)
			gtx.Constraints = layout.Exact(size)

			return widget.Image{
				Src:      a.menuButton,
				Fit:      widget.Cover,
				Position: layout.Center,
			}.Layout(gtx)
		})
	})
}

func (a *App) layoutCurrentView(gtx layout.Context) layout.Dimensions {
	switch a.mode {
	case ModeDiagram:
		return a.diagramView.Layout(gtx, a.theme)
	case ModeForward:
		return a.forwardView.Layout(gtx, a.theme)
	case ModeDictionary:
		return a.dictionaryView.Layout(gtx, a.theme)
	case ModeReverse:
		return a.reverseView.Layout(gtx, a.theme)
	}
	return layout.Dimensions{}
}
