package main

import (
	"log"
	"net/http"
)

func (a app) adminLoginHandler(w http.ResponseWriter, r *http.Request, token string) {
	switch r.Method {
	case http.MethodGet:
		renderTemplate(w, adminLoginTemplate, struct{ Error string }{})
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		if !constantTimeEqual(r.Form.Get("token"), token) {
			log.Printf("admin login failed ip=%s", clientIP(r))
			renderTemplate(w, adminLoginTemplate, struct{ Error string }{Error: "Invalid token"})
			return
		}
		setAdminSession(w, token)
		http.Redirect(w, r, "/admin/submissions", http.StatusSeeOther)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
