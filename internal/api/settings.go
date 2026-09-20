package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/eazy-gateway/eazy-gateway/internal/db"
)

// SettingsHandler handles GET /api/settings (public) and PUT /api/settings (authenticated).
func SettingsHandler(d *db.DB, sessions *SessionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			name, _ := d.GetSetting("appName")
			hours := 1
			if hStr, _ := d.GetSetting("trafficTrendHours"); hStr != "" {
				if v, err := strconv.Atoi(hStr); err == nil && v > 0 {
					hours = v
				}
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"appName":           name,
				"trafficTrendHours": hours,
			})

		case http.MethodPut:
			// Require session for updates
			cookie, err := r.Cookie(cookieName)
			if err == nil && cookie.Value != "" && sessions.Get(cookie.Value) {
				// ok
			} else if bt := extractBearerToken(r); bt != "" && sessions.Get(bt) {
				// ok
			} else {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}

			var body struct {
				AppName           string `json:"appName"`
				TrafficTrendHours *int   `json:"trafficTrendHours,omitempty"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
				return
			}
			if body.AppName != "" {
				if len(body.AppName) > 128 {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": "appName must not exceed 128 characters"})
					return
				}
				if err := d.SetSetting("appName", body.AppName); err != nil {
					log.Printf("ERROR: set appName setting: %v", err)
					writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
					return
				}
			}
			if body.TrafficTrendHours != nil {
				hours := *body.TrafficTrendHours
				if hours < 1 || hours > 24 {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": "trafficTrendHours must be between 1 and 24"})
					return
				}
				if err := d.SetSetting("trafficTrendHours", strconv.Itoa(hours)); err != nil {
					log.Printf("ERROR: set trafficTrendHours setting: %v", err)
					writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
					return
				}
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
