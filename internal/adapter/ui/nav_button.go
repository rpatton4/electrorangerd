package ui

import (
	"bytes"
	_ "embed"
	"image"
	_ "image/png"
	"log/slog"

	"gioui.org/op/paint"
)

//go:embed nav_button.png
var navButtonPNG []byte

// decodeNavButton decodes the embedded PNG into a paint.ImageOp. Returns a
// zero-value op on decode failure (paints nothing) and logs the error —
// callers will see a blank circle rather than a crash.
func decodeNavButton(log *slog.Logger) paint.ImageOp {
	img, _, err := image.Decode(bytes.NewReader(navButtonPNG))
	if err != nil {
		log.Error("decode nav button", "err", err)
		return paint.ImageOp{}
	}
	return paint.NewImageOp(img)
}
