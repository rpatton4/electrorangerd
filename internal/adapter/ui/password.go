package ui

import (
	"context"
	"errors"
	"image"
	"log/slog"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/rpatton4/electrorangerd/internal/adapter/ui/theme"
	"github.com/rpatton4/electrorangerd/internal/domain"
	"github.com/rpatton4/electrorangerd/internal/errs"
	"github.com/rpatton4/electrorangerd/internal/port"
)

// passwordPhase tracks which step of the master-password flow the user is on.
type passwordPhase int

const (
	pwPhaseSetup    passwordPhase = iota // vault Uninitialized — set new master password
	pwPhaseUnlock                        // vault Locked — prompt for existing master password
	pwPhaseUnlocked                      // proceed to main UI
)

// vaultDoorAnimDuration is how long the vault-door open animation
// takes — same value as infomenu's open duration so the login
// screen and the mode wheel animate in lockstep visually.
const vaultDoorAnimDuration = 500 * time.Millisecond

// passwordView renders the master-password setup / unlock screen. It calls
// port.Vault directly so the App's main loop only has to ask "are we
// unlocked yet?" by looking at vault.Status().
type passwordView struct {
	vault port.Vault
	log   *slog.Logger

	editor          widget.Editor
	vaultClick      widget.Clickable
	editorWrapClick widget.Clickable

	phase          passwordPhase
	lastErr        string
	focusRequested bool

	// Vault-door focal image plus the timestamp the screen first
	// rendered, so the door animates in (scale + rotate) once when
	// the login surface appears.
	vaultDoor paint.ImageOp
	animStart time.Time
}

func newPasswordView(vault port.Vault, log *slog.Logger) *passwordView {
	v := &passwordView{
		vault:     vault,
		log:       log,
		vaultDoor: decodeVaultDoor(log),
	}
	v.editor.SingleLine = true
	v.editor.Mask = '•'
	v.editor.Submit = true
	v.recomputePhase()
	return v
}

// recomputePhase aligns the view's phase with the vault's current status.
// Called on construction and after every Unlock/Initialize attempt.
func (v *passwordView) recomputePhase() {
	switch v.vault.Status() {
	case domain.VaultStatusUninitialized:
		v.phase = pwPhaseSetup
	case domain.VaultStatusLocked:
		v.phase = pwPhaseUnlock
	case domain.VaultStatusUnlocked:
		v.phase = pwPhaseUnlocked
	}
}

// tryKeychainUnlock attempts the opt-in OS-keychain auto-unlock. Called once
// before the first frame so the user is not prompted if the keychain holds a
// valid DEK from a previous session.
func (v *passwordView) tryKeychainUnlock(ctx context.Context) {
	if v.vault.Status() != domain.VaultStatusLocked {
		return
	}
	if err := v.vault.TryKeychainUnlock(ctx); err != nil {
		// Keychain unavailable or no entry — fall through to the password
		// prompt. This is the expected path on first launch.
		return
	}
	v.recomputePhase()
}

// Done reports whether the user can proceed to the main UI.
func (v *passwordView) Done() bool {
	return v.phase == pwPhaseUnlocked
}

// Layout draws the appropriate prompt for the current phase. It returns
// layout.Dimensions{} when the view should not be drawn (Done == true).
// The vault-door image sits at the centre of the screen with the input
// box overlaid on its centre; the door itself is clickable and acts as
// the submit action — pressing Enter inside the editor does the same.
// Any error from the previous attempt renders just below the door.
func (v *passwordView) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	if v.phase == pwPhaseUnlocked {
		return layout.Dimensions{}
	}

	// Submission: Enter inside the editor OR a click on the vault door
	// outside the editor's region. The editor sits on top of the door
	// in a Stack and we wrap it in editorWrapClick so we can detect
	// when a click landed on the textbox — those clicks consume the
	// editorWrapClick event AND would otherwise also fire vaultClick
	// (both input areas overlap). When that happens we suppress the
	// vault-door submission so typing-related clicks don't try to
	// unlock the vault.
	editorClickedThisFrame := false
	for v.editorWrapClick.Clicked(gtx) {
		editorClickedThisFrame = true
	}
	vaultClickedThisFrame := false
	for v.vaultClick.Clicked(gtx) {
		vaultClickedThisFrame = true
	}

	submitted := false
	for {
		ev, ok := v.editor.Update(gtx)
		if !ok {
			break
		}
		if _, isSubmit := ev.(widget.SubmitEvent); isSubmit {
			submitted = true
		}
	}
	if vaultClickedThisFrame && !editorClickedThisFrame {
		submitted = true
	}
	if submitted {
		v.handleSubmit()
	}

	if !v.focusRequested {
		gtx.Execute(key.FocusCmd{Tag: &v.editor})
		v.focusRequested = true
	}

	if v.animStart.IsZero() {
		v.animStart = gtx.Now
	}

	hint := "insert your key"
	if v.phase == pwPhaseSetup {
		hint = "new key"
	}

	return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Stack{Alignment: layout.Center}.Layout(gtx,
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						return v.vaultClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return v.layoutVaultDoor(gtx)
						})
					}),
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Max.X = gtx.Dp(unit.Dp(280))
						return v.editorWrapClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return v.layoutEditor(gtx, th, hint)
						})
					}),
				)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if v.lastErr == "" {
				return layout.Dimensions{}
			}
			errLbl := material.Body2(th.Material, v.lastErr)
			errLbl.Color = th.Error
			errLbl.Alignment = text.Middle
			return layout.Inset{Top: unit.Dp(12)}.Layout(gtx, errLbl.Layout)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(40)}.Layout),
	)
}

// layoutVaultDoor renders the vault-door image at 400dp square with
// the same scale-from-1/8 + rotate-360°-over-500ms open animation as
// the mode wheel. Animation start is recorded on the first Layout
// call so the door animates in exactly once per session.
func (v *passwordView) layoutVaultDoor(gtx layout.Context) layout.Dimensions {
	sz := gtx.Dp(unit.Dp(400))
	size := image.Pt(sz, sz)
	gtx.Constraints = layout.Exact(size)

	elapsed := gtx.Now.Sub(v.animStart)
	progress := float32(elapsed) / float32(vaultDoorAnimDuration)
	if progress >= 1 {
		progress = 1
	} else if progress < 0 {
		progress = 0
	}
	if progress < 1 {
		gtx.Execute(op.InvalidateCmd{})
	}

	scale := 0.125 + 0.875*progress
	rotation := progress * 2 * float32(math.Pi)
	centre := f32.Pt(float32(sz)/2, float32(sz)/2)
	affine := f32.Affine2D{}.
		Rotate(centre, rotation).
		Scale(centre, f32.Pt(scale, scale))
	defer op.Affine(affine).Push(gtx.Ops).Pop()

	return widget.Image{
		Src:      v.vaultDoor,
		Fit:      widget.Cover,
		Position: layout.Center,
	}.Layout(gtx)
}

// layoutEditor draws the password input with a visible bordered chrome, a
// focus-aware accent on the border, and an italic hint overlaid on the
// editor when the field is empty. The hint disappears as soon as the user
// types and reappears if they clear the field.
func (v *passwordView) layoutEditor(gtx layout.Context, th *theme.Theme, hintText string) layout.Dimensions {
	borderColor := th.Outline
	if gtx.Focused(&v.editor) {
		borderColor = th.Primary
	}
	border := widget.Border{
		Color:        borderColor,
		Width:        th.Sizes.StrokeThin,
		CornerRadius: th.Sizes.RadiusSM,
	}
	return border.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Stack{}.Layout(gtx,
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					ed := material.Editor(th.Material, &v.editor, "")
					return ed.Layout(gtx)
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					if v.editor.Text() != "" {
						return layout.Dimensions{}
					}
					hint := material.Body1(th.Material, hintText)
					hint.Font.Style = font.Italic
					hint.Color = th.OnSurfaceVariant
					return hint.Layout(gtx)
				}),
			)
		})
	})
}

func (v *passwordView) handleSubmit() {
	pw := v.editor.Text()
	if pw == "" {
		v.lastErr = "Master password cannot be empty."
		return
	}
	// Background context: the operation completes synchronously and the UI
	// blocks the frame on Argon2id derivation; acceptable for a security-
	// critical prompt where the user expects a moment of latency.
	ctx := context.Background()
	var err error
	switch v.phase {
	case pwPhaseSetup:
		err = v.vault.Initialize(ctx, pw)
	case pwPhaseUnlock:
		err = v.vault.Unlock(ctx, pw)
	}
	if err != nil {
		v.lastErr = friendlyError(err)
		v.log.Warn("master password rejected", "err", err)
		v.editor.SetText("")
		return
	}
	v.lastErr = ""
	v.editor.SetText("")
	v.recomputePhase()
}

func friendlyError(err error) string {
	switch {
	case errors.Is(err, errs.ErrInvalidPassword):
		return "Wrong password. Try again."
	case errors.Is(err, errs.ErrVaultAlreadyInitialized):
		return "Vault already exists. Restart to enter unlock mode."
	default:
		return "Could not complete the operation. Check the log for details."
	}
}
