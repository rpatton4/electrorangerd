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

	"github.com/InfiniteSkye/electrorangerd/internal/adapter/ui/infomenu"
	"github.com/InfiniteSkye/electrorangerd/internal/adapter/ui/panel"
	"github.com/InfiniteSkye/electrorangerd/internal/adapter/ui/theme"
	"github.com/InfiniteSkye/electrorangerd/internal/port"
)

// appName is the base OS window title. The window title also carries a
// mode suffix while the user is on the mode shell — see App.windowTitle.
const appName = "ElectroRangerD"

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
	forward       port.ForwardEngineer
	reverse       port.ReverseEngineer
	drift         port.DriftDetector
	project       port.ProjectService
	validator     port.Validator
	dictionary    port.DictionaryService
	diagramEditor port.DiagramEditor
	vault         port.Vault

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

	// Primary navigation — the reusable info-menu element (nav button
	// at screen-bottom + overlay wheel). Constructed in initViews so
	// it shares its lifetime with the rest of the per-frame views.
	infoMenu *infomenu.InfoMenu

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
	diagramEditor port.DiagramEditor,
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
		diagramEditor:     diagramEditor,
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
			app.Title(appName),
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
			w.Option(app.Title(a.windowTitle()))
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
				return a.infoMenu.LayoutInfoArea(gtx)
			}),
		)
		a.infoMenu.LayoutOverlay(gtx)
	}
}

// initViews builds the password gate, the welcome chooser, and the four
// per-mode views. Called once at the start of the first frame — the nil
// a.password serves as the "first frame" sentinel.
func (a *App) initViews() {
	a.password = newPasswordView(a.vault, a.log)
	a.infoMenu = infomenu.New(a.log, func(s infomenu.Sector) {
		a.mode = sectorToMode(s)
		a.screen = screenMode
	})
	a.welcomeView = newWelcomeView(a.infoMenu)
	a.diagramView = newDiagramView(a.diagramEditor, a.diagramHistory)
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

// windowTitle returns the OS window title to apply this frame. On the
// welcome screen it is the bare app name; on the mode shell it carries
// a " - <Mode> Mode" suffix so the title bar reflects the active mode.
func (a *App) windowTitle() string {
	if a.screen == screenMode {
		return appName + " - " + a.mode.String()
	}
	return appName
}

// sectorToMode maps an infomenu.Sector (the wheel's ring-section
// identifier) to the corresponding application Mode. Used by the
// InfoMenu's onSelect callback so the wheel can drive mode
// transitions without infomenu depending on the ui.Mode type.
func sectorToMode(s infomenu.Sector) Mode {
	switch s {
	case infomenu.SectorDiagram:
		return ModeDiagram
	case infomenu.SectorForward:
		return ModeForward
	case infomenu.SectorDictionary:
		return ModeDictionary
	case infomenu.SectorReverse:
		return ModeReverse
	default:
		return ModeDiagram
	}
}
