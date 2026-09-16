package httpapi

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/serverwatch/serverwatch/server/internal/service"
)

// callMonitorHandler exercises create/update validation. Every case below is
// rejected before the repository is used, so no database is required.
func callMonitorHandler(t *testing.T, handler *MonitorHandler, method string, body string) (int, string) {
	t.Helper()

	request := httptest.NewRequest(method, "/api/v1/monitors/1", strings.NewReader(body))
	request.SetPathValue("id", "1")

	recorder := httptest.NewRecorder()

	if method == http.MethodPut {
		handler.Update(recorder, request)
	} else {
		handler.Create(recorder, request)
	}

	return recorder.Code, recorder.Body.String()
}

func TestCreatePortMonitorValidation(t *testing.T) {
	twentyOnePorts := "1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21"

	tests := []struct {
		name    string
		body    string
		message string
	}{
		{
			"missing config",
			`{"name":"Ports","monitor_type":"port","target":"example.com"}`,
			"port config with ports is required",
		},
		{
			"empty ports",
			`{"name":"Ports","monitor_type":"port","target":"example.com","config":{"ports":[]}}`,
			"at least one port is required",
		},
		{
			"port zero",
			`{"name":"Ports","monitor_type":"port","target":"example.com","config":{"ports":[0]}}`,
			"ports must be between 1 and 65535",
		},
		{
			"port above range",
			`{"name":"Ports","monitor_type":"port","target":"example.com","config":{"ports":[65536]}}`,
			"ports must be between 1 and 65535",
		},
		{
			"negative port",
			`{"name":"Ports","monitor_type":"port","target":"example.com","config":{"ports":[-22]}}`,
			"ports must be between 1 and 65535",
		},
		{
			"too many ports",
			`{"name":"Ports","monitor_type":"port","target":"example.com","config":{"ports":[` + twentyOnePorts + `]}}`,
			"at most 20 ports",
		},
		{
			"range syntax",
			`{"name":"Ports","monitor_type":"port","target":"example.com","config":{"ports":["22-80"]}}`,
			"invalid_json",
		},
		{
			"range syntax list",
			`{"name":"Ports","monitor_type":"port","target":"example.com","config":{"ports":["22","80-443"]}}`,
			"invalid_json",
		},
		{
			"fractional port",
			`{"name":"Ports","monitor_type":"port","target":"example.com","config":{"ports":[443.5]}}`,
			"invalid_json",
		},
		{
			"non numeric port",
			`{"name":"Ports","monitor_type":"port","target":"example.com","config":{"ports":["abc"]}}`,
			"invalid_json",
		},
		{
			"url target",
			`{"name":"Ports","monitor_type":"port","target":"https://example.com","config":{"ports":[443]}}`,
			"invalid port target",
		},
		{
			"target with embedded port",
			`{"name":"Ports","monitor_type":"port","target":"example.com:443","config":{"ports":[443]}}`,
			"invalid port target",
		},
		{
			"target with path",
			`{"name":"Ports","monitor_type":"port","target":"example.com/health","config":{"ports":[443]}}`,
			"invalid port target",
		},
		{
			"target with query",
			`{"name":"Ports","monitor_type":"port","target":"example.com?x=1","config":{"ports":[443]}}`,
			"invalid port target",
		},
		{
			"target with whitespace",
			`{"name":"Ports","monitor_type":"port","target":"exa mple.com","config":{"ports":[443]}}`,
			"invalid port target",
		},
		{
			"invalid ip literal",
			`{"name":"Ports","monitor_type":"port","target":"999.999.999.999","config":{"ports":[443]}}`,
			"invalid port target",
		},
	}

	for _, test := range tests {
		handler := &MonitorHandler{}
		status, body := callMonitorHandler(t, handler, http.MethodPost, test.body)

		if status != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400 (%s)", test.name, status, body)
		}
		if !strings.Contains(body, test.message) {
			t.Fatalf("%s: body = %s, want message %q", test.name, body, test.message)
		}
	}
}

func TestUpdatePortMonitorValidation(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		message string
	}{
		{
			"missing config",
			`{"name":"Ports","monitor_type":"port","target":"example.com","interval_seconds":60,"timeout_seconds":10,"enabled":true}`,
			"port config with ports is required",
		},
		{
			"port above range",
			`{"name":"Ports","monitor_type":"port","target":"example.com","interval_seconds":60,"timeout_seconds":10,"enabled":true,"config":{"ports":[70000]}}`,
			"ports must be between 1 and 65535",
		},
		{
			"target with embedded port",
			`{"name":"Ports","monitor_type":"port","target":"example.com:22","interval_seconds":60,"timeout_seconds":10,"enabled":true,"config":{"ports":[22]}}`,
			"invalid port target",
		},
	}

	for _, test := range tests {
		handler := &MonitorHandler{}
		status, body := callMonitorHandler(t, handler, http.MethodPut, test.body)

		if status != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400 (%s)", test.name, status, body)
		}
		if !strings.Contains(body, test.message) {
			t.Fatalf("%s: body = %s, want message %q", test.name, body, test.message)
		}
	}
}

func TestValidatePortConfigNormalizesPorts(t *testing.T) {
	config := &service.MonitorConfig{Ports: []int{443, 80, 443, 22}}

	if err := validatePortConfig(config, "example.com"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []int{22, 80, 443}
	if !reflect.DeepEqual(config.Ports, want) {
		t.Fatalf("ports = %v, want %v", config.Ports, want)
	}
}

func TestValidatePortConfigAcceptsTargets(t *testing.T) {
	for _, target := range []string{"example.com", "192.168.1.10", "2001:db8::10", "localhost"} {
		config := &service.MonitorConfig{Ports: []int{22}}

		if err := validatePortConfig(config, target); err != nil {
			t.Fatalf("target %q: unexpected error: %v", target, err)
		}
	}
}

func TestValidatePortConfigRequiresConfig(t *testing.T) {
	if err := validatePortConfig(nil, "example.com"); err == nil {
		t.Fatal("expected error for missing port config")
	}
}
