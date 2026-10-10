package httpapi

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/serverwatch/serverwatch/server/internal/service"
)

func testUser(id int64) service.User {
	return service.User{ID: id, Email: "audit@example.com", IsAdmin: true, CreatedAt: time.Now()}
}

func TestAuditLogRecordsMutationsWithActor(t *testing.T) {
	var output bytes.Buffer
	audit := &AuditLog{Logger: log.New(&output, "", 0)}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	handler := audit.Middleware(inner)

	user := testUser(7)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/monitors", nil)
	request = request.WithContext(withPrincipal(request.Context(), principal{User: &user}))
	handler.ServeHTTP(httptest.NewRecorder(), request)

	line := output.String()
	if !strings.Contains(line, "method=POST") ||
		!strings.Contains(line, "path=/api/v1/monitors") ||
		!strings.Contains(line, "status=201") ||
		!strings.Contains(line, "actor=user") ||
		!strings.Contains(line, "user_id=7") {
		t.Fatalf("audit line missing expected fields: %q", line)
	}
}

func TestAuditLogSkipsReads(t *testing.T) {
	var output bytes.Buffer
	audit := &AuditLog{Logger: log.New(&output, "", 0)}
	handler := audit.Middleware(okHandler())

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/monitors", nil))

	if output.Len() != 0 {
		t.Fatalf("reads should not be audited, got %q", output.String())
	}
}

func TestAuditLogReportsTokenActor(t *testing.T) {
	var output bytes.Buffer
	audit := &AuditLog{Logger: log.New(&output, "", 0)}
	handler := audit.Middleware(okHandler())

	request := httptest.NewRequest(http.MethodDelete, "/api/v1/monitors/1", nil)
	request = request.WithContext(withPrincipal(request.Context(), principal{ViaToken: true}))
	handler.ServeHTTP(httptest.NewRecorder(), request)

	if !strings.Contains(output.String(), "actor=token") {
		t.Fatalf("token actor not recorded: %q", output.String())
	}
}

func TestAuditLogRecordsAnonymous(t *testing.T) {
	var output bytes.Buffer
	audit := &AuditLog{Logger: log.New(&output, "", 0)}
	handler := audit.Middleware(okHandler())

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/v1/groups", nil))

	if !strings.Contains(output.String(), "actor=anonymous") {
		t.Fatalf("anonymous actor not recorded: %q", output.String())
	}
}
