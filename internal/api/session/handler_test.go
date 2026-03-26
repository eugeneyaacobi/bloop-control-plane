package session

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"bloop-control-plane/internal/service"
)

func TestLogoutClearsCookie(t *testing.T) {
	h := &Handler{Service: &service.SessionService{}, CookieName: "bloop_session", CookieSecure: true, CookieDomain: "example.com"}
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	w := httptest.NewRecorder()

	h.Logout(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, w.Code)
	}
	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("expected logout cookie to be set")
	}
	if cookies[0].Name != "bloop_session" || cookies[0].MaxAge != -1 {
		t.Fatalf("expected clearing cookie, got %+v", cookies[0])
	}
	if !cookies[0].Secure {
		t.Fatalf("expected secure cookie to remain secure")
	}
}
