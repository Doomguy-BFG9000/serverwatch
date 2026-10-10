package httpapi

import "net/http"

// SecurityHeaders sets conservative, widely compatible response headers on
// every response. They mitigate common browser-side risks (MIME sniffing,
// framing, referrer leakage, protocol downgrade) without requiring any
// configuration, and are safe for both the JSON API and the static dashboard.
//
// The Content-Security-Policy is intentionally strict: the bundled dashboard
// is a static Vite build with no inline scripts, no external origins, and no
// eval, so 'self' alone is sufficient. If a future build needs inline styles or
// scripts, relax only the specific directive that requires it.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("X-Frame-Options", "DENY")
		header.Set("Referrer-Policy", "no-referrer")
		header.Set("Cross-Origin-Opener-Policy", "same-origin")
		header.Set("Cross-Origin-Resource-Policy", "same-origin")
		header.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		header.Set(
			"Content-Security-Policy",
			"default-src 'self'; "+
				"base-uri 'self'; "+
				"frame-ancestors 'none'; "+
				"form-action 'self'; "+
				"img-src 'self' data:; "+
				"style-src 'self'; "+
				"script-src 'self'; "+
				"connect-src 'self'",
		)
		// HSTS is meaningful only over HTTPS. Sending it on plain HTTP is
		// harmless for clients but can lock a misconfigured browser out of a
		// LAN host, so it is gated on the request arriving via TLS. Nginx
		// terminates TLS in the documented deployment and forwards the scheme.
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			header.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}
