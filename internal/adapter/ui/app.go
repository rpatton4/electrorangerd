package ui

import (
	"log/slog"
	"os"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/InfiniteSkye/electrorangerd/internal/port"
)

// App is the root of the GIOUI adapter. It holds references to every inbound
// port interface so that event handlers can delegate domain work without
// importing concrete implementations.
type App struct {
	forward    port.ForwardEngineer
	reverse    port.ReverseEngineer
	drift      port.DriftDetector
	project    port.ProjectService
	validator  port.Validator
	history    port.History
	dictionary port.DictionaryService
	vault      port.Vault
	log        *slog.Logger
	theme      *Theme
}

// NewApp constructs the App with all required port dependencies and a freshly
// created theme. It does not open a window; call Run to start the event loop.
func NewApp(
	fwd port.ForwardEngineer,
	rev port.ReverseEngineer,
	drift port.DriftDetector,
	proj port.ProjectService,
	val port.Validator,
	hist port.History,
	dict port.DictionaryService,
	vault port.Vault,
	log *slog.Logger,
) *App {
	return &App{
		forward:    fwd,
		reverse:    rev,
		drift:      drift,
		project:    proj,
		validator:  val,
		history:    hist,
		dictionary: dict,
		vault:      vault,
		log:        log,
		theme:      NewTheme(),
	}
}

// Run launches the Gio event loop. The window goroutine is started first, then
// app.Main() is called on the calling goroutine (which must be the OS main
// thread on macOS). Run only returns if the window loop exits with an error
// before os.Exit is called.
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

// loop is the per-window event loop. It runs on the window goroutine until the
// window is closed or an unrecoverable error occurs.
func (a *App) loop(w *app.Window) error {
	var ops op.Ops
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			layout.Center.Layout(gtx, material.Body1(a.theme.Material, "ElectroRangerD").Layout)
			e.Frame(gtx.Ops)
		}
	}
}
