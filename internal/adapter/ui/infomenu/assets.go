package infomenu

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

//go:embed nav_button.png
var navButtonPNG []byte

//go:embed menu_button.png
var menuButtonPNG []byte

//go:embed menu_button_diagram.png
var menuButtonDiagramPNG []byte

//go:embed menu_button_forward.png
var menuButtonForwardPNG []byte

//go:embed menu_button_dictionary.png
var menuButtonDictionaryPNG []byte

//go:embed menu_button_reverse.png
var menuButtonReversePNG []byte

// decodeNavButton decodes the embedded chrome PNG used as the small
// circular nav button. Returns a zero-value op on failure (paints
// nothing) and logs the error.
func decodeNavButton(log *slog.Logger) paint.ImageOp {
	img, _, err := image.Decode(bytes.NewReader(navButtonPNG))
	if err != nil {
		log.Error("decode nav button", "err", err)
		return paint.ImageOp{}
	}
	return paint.NewImageOp(img)
}

// decodeMenuButton decodes the embedded PNG used as the larger info
// menu wheel and applies an inscribed-circle alpha mask (1px
// anti-aliased edge) so renderers can draw it without a runtime
// clip — Gemini-generated PNGs have opaque grey corners, this masks
// them out at decode time.
func decodeMenuButton(log *slog.Logger) paint.ImageOp {
	return decodeMaskedPNG(log, menuButtonPNG, "menu button")
}

// decodeHighlightImages decodes the four per-sector highlight PNGs and
// returns them keyed by Sector. Each is masked with the same inscribed
// circle as the base wheel so renderers can swap between them without
// any per-image clip setup. A failed decode logs and leaves that
// sector's entry zero — renderWheel treats a zero ImageOp as "fall
// back to the base wheel".
func decodeHighlightImages(log *slog.Logger) map[Sector]paint.ImageOp {
	return map[Sector]paint.ImageOp{
		SectorDiagram:    decodeMaskedPNG(log, menuButtonDiagramPNG, "menu button diagram"),
		SectorForward:    decodeMaskedPNG(log, menuButtonForwardPNG, "menu button forward"),
		SectorDictionary: decodeMaskedPNG(log, menuButtonDictionaryPNG, "menu button dictionary"),
		SectorReverse:    decodeMaskedPNG(log, menuButtonReversePNG, "menu button reverse"),
	}
}

// decodeMaskedPNG decodes a single embedded PNG and applies the
// inscribed-circle alpha mask. Returns a zero ImageOp on failure.
func decodeMaskedPNG(log *slog.Logger, data []byte, label string) paint.ImageOp {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		log.Error("decode "+label, "err", err)
		return paint.ImageOp{}
	}
	return paint.NewImageOp(ApplyCircleMask(src))
}

// ApplyCircleMask returns a copy of src with every pixel outside the
// inscribed circle punched to alpha=0, with a 1px anti-aliased edge
// band around the boundary. The inscribed circle is centred at the
// image's centre and has radius min(width/2, height/2). Reusable by
// any caller in the ui tree that wants the same circular shaping
// the info-menu wheel uses at decode time.
func ApplyCircleMask(src image.Image) *image.NRGBA {
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
				continue
			}
			c := nsrc.NRGBAAt(b.Min.X+x, b.Min.Y+y)
			if fade < 0.5 {
				c.A = uint8(float64(c.A) * (fade + 0.5))
			}
			dst.SetNRGBA(b.Min.X+x, b.Min.Y+y, c)
		}
	}
	return dst
}
