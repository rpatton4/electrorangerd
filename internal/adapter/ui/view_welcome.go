package ui

import (
	"time"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/rpatton4/electrorangerd/internal/adapter/ui/infomenu"
	"github.com/rpatton4/electrorangerd/internal/adapter/ui/theme"
)

// welcomeView is the initial post-unlock chooser. It displays the
// mode wheel — the same chrome dial with the four mode labels that
// the info menu surfaces as an overlay on mode screens — centred in
// the window, scaled and rotated in on first frame the same way the
// overlay animates open. The wheel stays visible until a mode is
// selected; mode selection wiring lands in a future iteration.
type welcomeView struct {
	infoMenu *infomenu.InfoMenu

	// shownAt is set on the first Layout call so the wheel animates
	// in just once when welcome first appears. Zero value until that
	// first frame.
	shownAt time.Time
}

func newWelcomeView(menu *infomenu.InfoMenu) *welcomeView {
	return &welcomeView{infoMenu: menu}
}

func (v *welcomeView) Layout(gtx layout.Context, _ *theme.Theme) layout.Dimensions {
	if v.shownAt.IsZero() {
		v.shownAt = gtx.Now
	}
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return v.infoMenu.LayoutWheel(gtx, unit.Dp(400), v.shownAt)
	})
}
