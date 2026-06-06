package ui

import (
	"bytes"
	_ "embed"
	"image"
	_ "image/png"
	"log/slog"

	"gioui.org/op/paint"

	"github.com/InfiniteSkye/electrorangerd/internal/adapter/ui/infomenu"
)

//go:embed vault_door.png
var vaultDoorPNG []byte

// decodeVaultDoor decodes the embedded vault-door PNG used as the
// focal element on the login screen, applying the same
// inscribed-circle alpha mask the info-menu wheel uses so the
// corners of the bounding square are punched to transparent at
// decode time. Returns a zero-value op on failure (paints nothing)
// and logs the error.
func decodeVaultDoor(log *slog.Logger) paint.ImageOp {
	img, _, err := image.Decode(bytes.NewReader(vaultDoorPNG))
	if err != nil {
		log.Error("decode vault door", "err", err)
		return paint.ImageOp{}
	}
	return paint.NewImageOp(infomenu.ApplyCircleMask(img))
}
