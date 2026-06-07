package ui

import (
	"bytes"
	_ "embed"
	"image"
	_ "image/png"
	"log/slog"

	"gioui.org/op/paint"
)

//go:embed erd_lost.png
var erdLostPNG []byte

// decodeErdLost decodes the embedded "lost designer" PNG shown
// inside the maze-of-twisty-passages notice. Rectangular, no mask
// applied. Returns a zero-value op on failure (paints nothing) and
// logs the error.
func decodeErdLost(log *slog.Logger) paint.ImageOp {
	img, _, err := image.Decode(bytes.NewReader(erdLostPNG))
	if err != nil {
		log.Error("decode erd lost", "err", err)
		return paint.ImageOp{}
	}
	return paint.NewImageOp(img)
}
