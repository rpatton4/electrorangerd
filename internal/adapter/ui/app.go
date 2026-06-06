package ui

import (
	"context"
	"image"
	"image/color"
	"log/slog"
	"math"
	"os"
	"time"

	"gioui.org/app"
	"gioui.org/f32"
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

// menuOpenDuration is how long the menu-wheel open animation takes. Over
// this span the wheel scales from 1/8 to full size and rotates 360°.
const menuOpenDuration = 500 * time.Millisecond

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
	navClick     widget.Clickable
	navButton    paint.ImageOp
	menuOpen     bool
	menuOpenedAt time.Time
	menuClick    widget.Clickable
	menuButton   paint.ImageOp
	scrimClick   widget.Clickable

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
// it opens the menu overlay. Dismiss is handled exclusively by the
// scrim's click area (see layoutMenuOverlay); the nav button never
// closes the menu, so there's no double-toggle race between the two
// handlers on a single click.
func (a *App) layoutModeNav(gtx layout.Context) layout.Dimensions {
	if a.menuOpen {
		// While menu is up, drain any nav clicks that were captured
		// before the slot stopped registering, so they don't pile up.
		for a.navClick.Clicked(gtx) {
		}
	} else if a.navClick.Clicked(gtx) {
		a.menuOpen = true
		a.menuOpenedAt = gtx.Now
	}

	return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			sz := gtx.Dp(unit.Dp(56))
			size := image.Pt(sz, sz)

			// While the menu wheel is up the chrome button hides AND
			// stops registering its click area, so the scrim above is
			// the sole receiver of taps in the bottom-centre region.
			// The 56dp slot is still reserved so the layout doesn't
			// shift between open and closed states.
			if a.menuOpen {
				return layout.Dimensions{Size: size}
			}

			return a.navClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
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
// labels around the rim) floating centred on top of the mode shell. On
// open it scales from 1/8 to full size while rotating 360°; a near-black
// scrim fades in over the same span, muting the underlying UI so it
// stays visible but barely. The click is registered but currently does
// nothing; tap the bottom slot to dismiss.
func (a *App) layoutMenuOverlay(gtx layout.Context) layout.Dimensions {
	_ = a.menuClick.Clicked(gtx) // consume; menu does nothing yet
	if a.scrimClick.Clicked(gtx) {
		// Close the menu now AND skip painting the overlay this frame —
		// otherwise this frame still commits the scrim + wheel ops and
		// the user only sees the close on the next frame (which won't
		// fire until they click again). The invalidate makes the harness
		// schedule a follow-up frame so the layout below (nav button
		// re-registering its click area) settles properly.
		a.menuOpen = false
		gtx.Execute(op.InvalidateCmd{})
		return layout.Dimensions{}
	}

	elapsed := gtx.Now.Sub(a.menuOpenedAt)
	progress := float32(elapsed) / float32(menuOpenDuration)
	if progress >= 1 {
		progress = 1
	} else if progress < 0 {
		progress = 0
	}
	if progress < 1 {
		gtx.Execute(op.InvalidateCmd{})
	}

	// Scrim — semi-opaque near-black across the whole window so the
	// mode shell behind reads as muted. Alpha fades in with the open
	// animation rather than snapping on, to match the wheel's spin-in.
	// Wrapped in scrimClick so any tap on it (i.e. outside the wheel,
	// which sits on top with its own clickable) dismisses the menu.
	a.scrimClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		scrim := color.NRGBA{A: uint8(float32(0xD8) * progress)}
		scrimRect := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
		paint.ColorOp{Color: scrim}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		scrimRect.Pop()
		return layout.Dimensions{Size: gtx.Constraints.Max}
	})

	scale := 0.125 + 0.875*progress
	rotation := progress * 2 * float32(math.Pi)

	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		sz := gtx.Dp(unit.Dp(320))
		size := image.Pt(sz, sz)

		// Restrict the menu's click area to the inscribed circle. Without
		// this, the four corners of the 320dp bounding square (visually
		// transparent — the geometric alpha mask makes them invisible)
		// would still capture taps as menu clicks (a no-op), preventing
		// the scrim from dismissing on what the user perceives as a tap
		// outside the wheel. Clip pushed before menuClick.Layout so its
		// gesture.Click is registered inside the elliptical clip.
		defer clip.Ellipse{Max: size}.Push(gtx.Ops).Pop()

		return a.menuClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints = layout.Exact(size)

			centre := f32.Pt(float32(sz)/2, float32(sz)/2)
			affine := f32.Affine2D{}.
				Rotate(centre, rotation).
				Scale(centre, f32.Pt(scale, scale))
			defer op.Affine(affine).Push(gtx.Ops).Pop()

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
