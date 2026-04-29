package handlers

import (
	"image/png"
	"net/http"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/skip2/go-qrcode"
	"library/internal/middleware"
)

func (h *Handler) GetQRCode(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	png, err := qrcode.Encode(claims.UserID, qrcode.High, 256)
	if err != nil {
		jsonErr(w, 500, "failed to generate QR code")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Write(png)
}

func (h *Handler) GetBarcode(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)

	// Truncate UUID to 20 chars so it fits nicely in Code128
	id := claims.UserID
	if len(id) > 20 {
		id = id[:20]
	}

	bc, err := code128.Encode(id)
	if err != nil {
		jsonErr(w, 500, "failed to generate barcode")
		return
	}
	scaled, err := barcode.Scale(bc, 400, 80)
	if err != nil {
		jsonErr(w, 500, "failed to scale barcode")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	png.Encode(w, scaled)
}
