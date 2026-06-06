package ui

import (
	"context"
	"errors"
	"log/slog"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/InfiniteSkye/electrorangerd/internal/adapter/ui/theme"
	"github.com/InfiniteSkye/electrorangerd/internal/domain"
	"github.com/InfiniteSkye/electrorangerd/internal/errs"
	"github.com/InfiniteSkye/electrorangerd/internal/port"
)

// passwordPhase tracks which step of the master-password flow the user is on.
type passwordPhase int

const (
	pwPhaseSetup    passwordPhase = iota // vault Uninitialized — set new master password
	pwPhaseUnlock                        // vault Locked — prompt for existing master password
	pwPhaseUnlocked                      // proceed to main UI
)

// passwordView renders the master-password setup / unlock screen. It calls
// port.Vault directly so the App's main loop only has to ask "are we
// unlocked yet?" by looking at vault.Status().
type passwordView struct {
	vault port.Vault
	log   *slog.Logger

	editor widget.Editor
	submit widget.Clickable

	phase          passwordPhase
	lastErr        string
	focusRequested bool
}

func newPasswordView(vault port.Vault, log *slog.Logger) *passwordView {
	v := &passwordView{vault: vault, log: log}
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
func (v *passwordView) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	if v.phase == pwPhaseUnlocked {
		return layout.Dimensions{}
	}

	// Handle Enter-key submit and button click identically.
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
	if v.submit.Clicked(gtx) {
		submitted = true
	}
	if submitted {
		v.handleSubmit()
	}

	if !v.focusRequested {
		gtx.Execute(key.FocusCmd{Tag: &v.editor})
		v.focusRequested = true
	}

	title := "unlock the vault"
	help := ""
	button := "Unlock"
	hint := "insert your key"
	if v.phase == pwPhaseSetup {
		title = "welcome to your design grid"
		help = "choose your key, this will encrypt all sensitive info"
		button = "Create vault"
		hint = "new key"
	}

	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.X = gtx.Dp(unit.Dp(480))
		return layout.UniformInset(unit.Dp(24)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(material.H5(th.Material, title).Layout),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if help == "" {
						return layout.Spacer{Height: unit.Dp(17)}.Layout(gtx)
					}
					return material.Body2(th.Material, help).Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(24)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return v.layoutEditor(gtx, th, hint)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
				layout.Rigid(material.Button(th.Material, &v.submit, button).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if v.lastErr == "" {
						return layout.Dimensions{}
					}
					errLbl := material.Body2(th.Material, v.lastErr)
					errLbl.Color = th.Error
					return layout.Inset{Top: unit.Dp(12)}.Layout(gtx, errLbl.Layout)
				}),
			)
		})
	})
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
