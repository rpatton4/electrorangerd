package ui

import (
	"bytes"
	_ "embed"
	"image"
	"image/draw"
	_ "image/png"
	"log/slog"
	"math"

	"gioui.org/op/paint"
)

//go:embed menu_button.png
var menuButtonPNG []byte

// decodeMenuButton decodes the embedded PNG and applies an inscribed-circle
// alpha mask: every pixel outside the inscribed circle is punched to
// alpha=0, with a 1-pixel anti-aliased edge band so the boundary reads
// smooth rather than jagged. This works around the source PNG having
// opaque grey corners that no clip-at-render would have to compensate
// for. Returns a zero-value op on decode failure (paints nothing).
func decodeMenuButton(log *slog.Logger) paint.ImageOp {
	src, _, err := image.Decode(bytes.NewReader(menuButtonPNG))
	if err != nil {
		log.Error("decode menu button", "err", err)
		return paint.ImageOp{}
	}

	b := src.Bounds()
	nsrc := image.NewNRGBA(b)
	draw.Draw(nsrc, b, src, b.Min, draw.Src)

	dst := image.NewNRGBA(b)
	w, h := b.Dx(), b.Dy()
	cx := float64(w) / 2
	cy := float64(h) / 2
	r := math.Min(cx, cy)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx := float64(x) - cx + 0.5
			dy := float64(y) - cy + 0.5
			fade := r - math.Sqrt(dx*dx+dy*dy)
			if fade <= -0.5 {
				continue // outside: stays transparent
			}
			c := nsrc.NRGBAAt(b.Min.X+x, b.Min.Y+y)
			if fade < 0.5 {
				c.A = uint8(float64(c.A) * (fade + 0.5))
			}
			dst.SetNRGBA(b.Min.X+x, b.Min.Y+y, c)
		}
	}
	return paint.NewImageOp(dst)
}
