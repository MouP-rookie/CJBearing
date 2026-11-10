package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bearing-inspection/internal/auth"
	"github.com/gin-gonic/gin"
)

func TestSharedCookieAuthenticatesPortalSessionAcrossPorts(t *testing.T) {
	service := auth.NewService("sso-regression-secret", time.Hour)
	router := gin.New()
	router.Use(Auth(service), RequireClient(auth.ClientPC, auth.ClientPortal))
	router.GET("/admin", func(c *gin.Context) {
		if c.GetUint64("user_id") != 42 {
			t.Errorf("identity changed: %d", c.GetUint64("user_id"))
		}
		c.Status(http.StatusNoContent)
	})

	for _, test := range []struct {
		name   string
		client string
		want   int
	}{
		{"portal password session", auth.ClientPortal, http.StatusNoContent},
		{"legacy PC password session", auth.ClientPC, http.StatusNoContent},
		{"PDA employee-number session cannot access PC", auth.ClientPDA, http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			token, _, err := service.GenerateToken(42, "shared-user", test.client)
			if err != nil {
				t.Fatal(err)
			}
			for _, origin := range []string{"http://localhost:8090", "http://localhost:5173", "http://192.168.31.34:5173"} {
				request := httptest.NewRequest(http.MethodGet, origin+"/admin", nil)
				request.AddCookie(&http.Cookie{Name: "token", Value: token, Path: "/", HttpOnly: true})
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)
				if response.Code != test.want {
					t.Errorf("%s: got %d, want %d: %s", origin, response.Code, test.want, response.Body.String())
				}
				if len(response.Result().Cookies()) != 0 {
					t.Fatal("entering a subsystem must not overwrite the shared login cookie")
				}
			}
		})
	}
	for _, cookie := range []string{"", "invalid-token"} {
		request := httptest.NewRequest(http.MethodGet, "/admin", nil)
		request.AddCookie(&http.Cookie{Name: "token", Value: cookie})
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("invalid session: got %d, want 401", response.Code)
		}
	}
}
