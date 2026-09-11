package server

import (
	"net/http"
	"strings"
)

func (s *Server) validatePromo(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TariffID  string `json:"tariff_id"`
		PromoCode string `json:"promo_code"`
	}
	if !decode(w, r, &body) {
		return
	}
	features, _ := s.Store.Setting(r.Context(), "features")
	if !boolean(features["promo_codes"]) {
		writeError(w, http.StatusBadRequest, "Промокоды временно отключены")
		return
	}
	tariff, err := s.Store.Tariff(r.Context(), strings.TrimSpace(body.TariffID))
	if err != nil || !tariff.Active {
		writeError(w, http.StatusNotFound, "Тариф не найден")
		return
	}
	promo, err := s.Store.PromoByCode(r.Context(), body.PromoCode)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Промокод не найден")
		return
	}
	if err := validPromo(promo); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	amount := discounted(tariff.PriceKopecks, promo.DiscountPercent)
	writeJSON(w, http.StatusOK, map[string]any{
		"code":               promo.Code,
		"discount_percent":   promo.DiscountPercent,
		"original_price_rub": float64(tariff.PriceKopecks) / 100,
		"price_rub":          float64(amount) / 100,
	})
}
