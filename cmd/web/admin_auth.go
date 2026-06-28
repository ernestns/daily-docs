package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

func (a app) adminHandler(w http.ResponseWriter, r *http.Request) {
	token := os.Getenv("ADMIN_TOKEN")
	if token == "" {
		http.NotFound(w, r)
		return
	}

	path := strings.TrimSuffix(r.URL.Path, "/")
	if path == "" {
		path = "/admin"
	}

	if path == "/admin/login" {
		a.adminLoginHandler(w, r, token)
		return
	}

	if !validAdminSession(r, token) {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return
	}

	switch {
	case path == "/admin":
		http.Redirect(w, r, "/admin/submissions", http.StatusSeeOther)
	case path == "/admin/sources":
		a.adminSourcesHandler(w, r, token)
	case strings.HasPrefix(path, "/admin/sources/"):
		a.adminSourceHandler(w, r, token, path)
	case strings.HasPrefix(path, "/admin/runs/"):
		a.adminRunHandler(w, r, path)
	case path == "/admin/submissions":
		a.adminSubmissionsHandler(w, r, token)
	case strings.HasPrefix(path, "/admin/submissions/"):
		a.adminSubmissionHandler(w, r, token, path)
	default:
		http.NotFound(w, r)
	}
}

func redirectAdminSubmission(w http.ResponseWriter, r *http.Request, submissionID int64, notice string, errorMessage string) {
	values := url.Values{}
	if notice != "" {
		values.Set("notice", notice)
	}
	if errorMessage != "" {
		values.Set("error", errorMessage)
	}
	target := fmt.Sprintf("/admin/submissions/%d", submissionID)
	if encoded := values.Encode(); encoded != "" {
		target += "?" + encoded
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func redirectAdminSources(w http.ResponseWriter, r *http.Request, notice string, errorMessage string) {
	values := url.Values{}
	if notice != "" {
		values.Set("notice", notice)
	}
	if errorMessage != "" {
		values.Set("error", errorMessage)
	}
	target := "/admin/sources"
	if encoded := values.Encode(); encoded != "" {
		target += "?" + encoded
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func redirectAdminSource(w http.ResponseWriter, r *http.Request, sourceID int64, notice string, errorMessage string) {
	values := url.Values{}
	if notice != "" {
		values.Set("notice", notice)
	}
	if errorMessage != "" {
		values.Set("error", errorMessage)
	}
	target := fmt.Sprintf("/admin/sources/%d", sourceID)
	if encoded := values.Encode(); encoded != "" {
		target += "?" + encoded
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func setAdminSession(w http.ResponseWriter, token string) {
	issuedAt := strconv.FormatInt(time.Now().UTC().Unix(), 10)
	signature := signAdminValue(issuedAt, token)
	http.SetCookie(w, &http.Cookie{
		Name:     adminSessionCookie,
		Value:    issuedAt + "." + signature,
		Path:     "/admin",
		MaxAge:   int(adminSessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

func validAdminSession(r *http.Request, token string) bool {
	cookie, err := r.Cookie(adminSessionCookie)
	if err != nil {
		return false
	}
	issuedAt, signature, ok := strings.Cut(cookie.Value, ".")
	if !ok || issuedAt == "" || signature == "" {
		return false
	}
	expected := signAdminValue(issuedAt, token)
	if !constantTimeEqual(signature, expected) {
		return false
	}
	issuedUnix, err := strconv.ParseInt(issuedAt, 10, 64)
	if err != nil {
		return false
	}
	issued := time.Unix(issuedUnix, 0)
	return time.Since(issued) >= 0 && time.Since(issued) <= adminSessionTTL
}

func csrfToken(r *http.Request, token string) string {
	cookie, err := r.Cookie(adminSessionCookie)
	if err != nil {
		return ""
	}
	return signAdminValue("csrf:"+cookie.Value, token)
}

func validCSRF(r *http.Request, token string) bool {
	if err := r.ParseForm(); err != nil {
		return false
	}
	return constantTimeEqual(r.Form.Get("csrf"), csrfToken(r, token))
}

func signAdminValue(value string, token string) string {
	mac := hmac.New(sha256.New, []byte(token))
	_, _ = mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

func constantTimeEqual(left string, right string) bool {
	return hmac.Equal([]byte(left), []byte(right))
}
