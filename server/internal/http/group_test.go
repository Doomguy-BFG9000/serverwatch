package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/serverwatch/serverwatch/server/internal/repository"
	"github.com/serverwatch/serverwatch/server/internal/service"
)

// fakeGroupMonitorStore implements both GroupStore and MonitorStore so the
// HTTP tests can exercise group and monitor integration deterministically
// without a live database. Database referential behavior itself is covered by
// migration 004's ON DELETE SET NULL declaration.
type fakeGroupMonitorStore struct {
	groups        map[int64]service.Group
	monitors      map[int64]service.Monitor
	nextGroupID   int64
	nextMonitorID int64
}

func newFakeGroupMonitorStore() *fakeGroupMonitorStore {
	return &fakeGroupMonitorStore{
		groups:        make(map[int64]service.Group),
		monitors:      make(map[int64]service.Monitor),
		nextGroupID:   1,
		nextMonitorID: 1,
	}
}

func (store *fakeGroupMonitorStore) CreateGroup(
	_ context.Context,
	input service.CreateGroupInput,
) (service.Group, error) {
	name, err := service.ValidateGroupName(input.Name)
	if err != nil {
		return service.Group{}, err
	}
	for _, group := range store.groups {
		if strings.EqualFold(group.Name, name) {
			return service.Group{}, repository.ErrGroupNameConflict
		}
	}

	group := service.Group{
		ID:        store.nextGroupID,
		Name:      name,
		CreatedAt: time.Unix(0, 0).UTC(),
		UpdatedAt: time.Unix(0, 0).UTC(),
	}
	store.nextGroupID++
	store.groups[group.ID] = group
	return group, nil
}

func (store *fakeGroupMonitorStore) ListGroups(
	_ context.Context,
) ([]service.Group, error) {
	groups := make([]service.Group, 0, len(store.groups))
	for _, group := range store.groups {
		groups = append(groups, group)
	}
	sort.Slice(groups, func(i, j int) bool {
		return strings.ToLower(groups[i].Name) < strings.ToLower(groups[j].Name)
	})
	return groups, nil
}

func (store *fakeGroupMonitorStore) UpdateGroup(
	_ context.Context,
	id int64,
	input service.UpdateGroupInput,
) (service.Group, error) {
	group, ok := store.groups[id]
	if !ok {
		return service.Group{}, repository.ErrGroupNotFound
	}

	name, err := service.ValidateGroupName(input.Name)
	if err != nil {
		return service.Group{}, err
	}
	for _, existing := range store.groups {
		if existing.ID != id && strings.EqualFold(existing.Name, name) {
			return service.Group{}, repository.ErrGroupNameConflict
		}
	}

	group.Name = name
	group.UpdatedAt = time.Unix(1, 0).UTC()
	store.groups[id] = group
	return group, nil
}

func (store *fakeGroupMonitorStore) DeleteGroup(
	_ context.Context,
	id int64,
) error {
	if _, ok := store.groups[id]; !ok {
		return repository.ErrGroupNotFound
	}

	delete(store.groups, id)
	for monitorID, monitor := range store.monitors {
		if monitor.GroupID != nil && *monitor.GroupID == id {
			monitor.GroupID = nil
			monitor.GroupName = ""
			store.monitors[monitorID] = monitor
		}
	}
	return nil
}

func (store *fakeGroupMonitorStore) Create(
	_ context.Context,
	input service.CreateMonitorInput,
) (service.Monitor, error) {
	monitor := service.Monitor{
		ID:              store.nextMonitorID,
		Name:            input.Name,
		MonitorType:     input.MonitorType,
		Target:          input.Target,
		Config:          input.Config,
		IntervalSeconds: input.IntervalSeconds,
		TimeoutSeconds:  input.TimeoutSeconds,
		Enabled:         true,
		GroupID:         input.GroupID,
		CreatedAt:       time.Unix(0, 0).UTC(),
		UpdatedAt:       time.Unix(0, 0).UTC(),
	}
	store.nextMonitorID++

	if monitor.GroupID != nil {
		group, ok := store.groups[*monitor.GroupID]
		if !ok {
			return service.Monitor{}, repository.ErrGroupNotFound
		}
		monitor.GroupName = group.Name
	}

	store.monitors[monitor.ID] = monitor
	return monitor, nil
}

func (store *fakeGroupMonitorStore) List(
	_ context.Context,
) ([]service.Monitor, error) {
	monitors := make([]service.Monitor, 0, len(store.monitors))
	for _, monitor := range store.monitors {
		monitors = append(monitors, monitor)
	}
	sort.Slice(monitors, func(i, j int) bool {
		return monitors[i].ID > monitors[j].ID
	})
	return monitors, nil
}

func (store *fakeGroupMonitorStore) GetByID(
	_ context.Context,
	id int64,
) (service.Monitor, error) {
	monitor, ok := store.monitors[id]
	if !ok {
		return service.Monitor{}, repository.ErrMonitorNotFound
	}
	return monitor, nil
}

func (store *fakeGroupMonitorStore) Delete(
	_ context.Context,
	id int64,
) error {
	if _, ok := store.monitors[id]; !ok {
		return repository.ErrMonitorNotFound
	}
	delete(store.monitors, id)
	return nil
}

func (store *fakeGroupMonitorStore) ListStatus(
	_ context.Context,
) ([]service.MonitorStatus, error) {
	statuses := make([]service.MonitorStatus, 0, len(store.monitors))
	for _, monitor := range store.monitors {
		statuses = append(statuses, service.MonitorStatus{
			ID:              monitor.ID,
			Name:            monitor.Name,
			MonitorType:     monitor.MonitorType,
			Target:          monitor.Target,
			Config:          monitor.Config,
			Enabled:         monitor.Enabled,
			Maintenance:     monitor.Maintenance,
			Favorite:        monitor.Favorite,
			IntervalSeconds: monitor.IntervalSeconds,
			TimeoutSeconds:  monitor.TimeoutSeconds,
			Status:          "pending",
			CreatedAt:       monitor.CreatedAt,
			GroupID:         monitor.GroupID,
			GroupName:       monitor.GroupName,
		})
	}
	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].ID > statuses[j].ID
	})
	return statuses, nil
}

func (store *fakeGroupMonitorStore) ListResultsSince(
	_ context.Context,
	_ int64,
	_ int,
	_ int,
) ([]service.MonitorResult, error) {
	return []service.MonitorResult{}, nil
}

func (store *fakeGroupMonitorStore) Update(
	_ context.Context,
	id int64,
	input service.UpdateMonitorInput,
) (service.Monitor, error) {
	monitor, ok := store.monitors[id]
	if !ok {
		return service.Monitor{}, repository.ErrMonitorNotFound
	}

	monitor.Name = input.Name
	monitor.MonitorType = input.MonitorType
	monitor.Target = input.Target
	monitor.Config = input.Config
	monitor.IntervalSeconds = input.IntervalSeconds
	monitor.TimeoutSeconds = input.TimeoutSeconds
	monitor.Enabled = input.Enabled

	if input.GroupID.Set {
		if input.GroupID.Value == nil {
			monitor.GroupID = nil
			monitor.GroupName = ""
		} else {
			group, ok := store.groups[*input.GroupID.Value]
			if !ok {
				return service.Monitor{}, repository.ErrGroupNotFound
			}
			groupID := *input.GroupID.Value
			monitor.GroupID = &groupID
			monitor.GroupName = group.Name
		}
	}

	if input.Maintenance.Set {
		monitor.Maintenance = input.Maintenance.Value
	}

	monitor.UpdatedAt = time.Unix(1, 0).UTC()
	store.monitors[id] = monitor
	return monitor, nil
}

func (store *fakeGroupMonitorStore) SetMaintenance(
	_ context.Context,
	id int64,
	maintenance bool,
) (service.Monitor, error) {
	monitor, ok := store.monitors[id]
	if !ok {
		return service.Monitor{}, repository.ErrMonitorNotFound
	}

	monitor.Maintenance = maintenance
	monitor.UpdatedAt = time.Unix(1, 0).UTC()
	store.monitors[id] = monitor
	return monitor, nil
}

func (store *fakeGroupMonitorStore) SetFavorite(
	_ context.Context,
	id int64,
	favorite bool,
) (service.Monitor, error) {
	monitor, ok := store.monitors[id]
	if !ok {
		return service.Monitor{}, repository.ErrMonitorNotFound
	}

	monitor.Favorite = favorite
	monitor.UpdatedAt = time.Unix(1, 0).UTC()
	store.monitors[id] = monitor
	return monitor, nil
}

func (store *fakeGroupMonitorStore) GetStatsSince(
	_ context.Context,
	monitorID int64,
	_ int,
) (service.MonitorStats, error) {
	return service.MonitorStats{MonitorID: monitorID}, nil
}

func (store *fakeGroupMonitorStore) ListChartPoints(
	_ context.Context,
	_ int64,
	_ int,
	_ int,
) ([]service.MonitorChartPoint, error) {
	return []service.MonitorChartPoint{}, nil
}

func callGroupHandler(
	t *testing.T,
	handler *GroupHandler,
	method string,
	id string,
	body string,
) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(method, "/api/v1/groups/"+id, strings.NewReader(body))
	request.SetPathValue("id", id)
	recorder := httptest.NewRecorder()

	switch method {
	case http.MethodPost:
		handler.Create(recorder, request)
	case http.MethodGet:
		handler.List(recorder, request)
	case http.MethodPut:
		handler.Update(recorder, request)
	case http.MethodDelete:
		handler.Delete(recorder, request)
	default:
		t.Fatalf("unsupported method %s", method)
	}

	return recorder
}

func TestGroupAPI(t *testing.T) {
	store := newFakeGroupMonitorStore()
	handler := &GroupHandler{Repository: store}

	created := callGroupHandler(t, handler, http.MethodPost, "", `{"name":"  Production  "}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", created.Code, created.Body.String())
	}
	var group service.Group
	if err := json.Unmarshal(created.Body.Bytes(), &group); err != nil {
		t.Fatalf("decode created group: %v", err)
	}
	if group.Name != "Production" {
		t.Fatalf("created name = %q, want Production", group.Name)
	}

	duplicate := callGroupHandler(t, handler, http.MethodPost, "", `{"name":"production"}`)
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d, body = %s", duplicate.Code, duplicate.Body.String())
	}

	listed := callGroupHandler(t, handler, http.MethodGet, "", "")
	if listed.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listed.Code, listed.Body.String())
	}
	var groups []service.Group
	if err := json.Unmarshal(listed.Body.Bytes(), &groups); err != nil {
		t.Fatalf("decode groups: %v", err)
	}
	if len(groups) != 1 || groups[0].Name != "Production" {
		t.Fatalf("groups = %+v, want one Production group", groups)
	}

	updated := callGroupHandler(t, handler, http.MethodPut, "1", `{"name":"Core Infrastructure"}`)
	if updated.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", updated.Code, updated.Body.String())
	}

	second := callGroupHandler(t, handler, http.MethodPost, "", `{"name":"Network"}`)
	if second.Code != http.StatusCreated {
		t.Fatalf("create second group status = %d, body = %s", second.Code, second.Body.String())
	}
	renameConflict := callGroupHandler(t, handler, http.MethodPut, "2", `{"name":"core infrastructure"}`)
	if renameConflict.Code != http.StatusConflict {
		t.Fatalf("rename conflict status = %d, body = %s", renameConflict.Code, renameConflict.Body.String())
	}

	malformed := callGroupHandler(t, handler, http.MethodPut, "not-a-number", `{"name":"Nope"}`)
	if malformed.Code != http.StatusBadRequest {
		t.Fatalf("malformed status = %d, body = %s", malformed.Code, malformed.Body.String())
	}

	missing := callGroupHandler(t, handler, http.MethodPut, "999", `{"name":"Nope"}`)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing update status = %d, body = %s", missing.Code, missing.Body.String())
	}

	deleted := callGroupHandler(t, handler, http.MethodDelete, "1", "")
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, body = %s", deleted.Code, deleted.Body.String())
	}

	missingDelete := callGroupHandler(t, handler, http.MethodDelete, "1", "")
	if missingDelete.Code != http.StatusNotFound {
		t.Fatalf("missing delete status = %d, body = %s", missingDelete.Code, missingDelete.Body.String())
	}
}

func TestGroupAPIValidation(t *testing.T) {
	handler := &GroupHandler{Repository: newFakeGroupMonitorStore()}

	tests := []struct {
		name string
		body string
	}{
		{"missing name", `{}`},
		{"empty name", `{"name":""}`},
		{"whitespace only", `{"name":"   "}`},
		{"too long", `{"name":"` + strings.Repeat("a", service.MaxGroupNameLength+1) + `"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := callGroupHandler(t, handler, http.MethodPost, "", test.body)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestMonitorGroupIntegration(t *testing.T) {
	store := newFakeGroupMonitorStore()
	groupHandler := &GroupHandler{Repository: store}
	monitorHandler := &MonitorHandler{Repository: store}

	production := callGroupHandler(t, groupHandler, http.MethodPost, "", `{"name":"Production"}`)
	if production.Code != http.StatusCreated {
		t.Fatalf("create production group: %s", production.Body.String())
	}
	network := callGroupHandler(t, groupHandler, http.MethodPost, "", `{"name":"Network"}`)
	if network.Code != http.StatusCreated {
		t.Fatalf("create network group: %s", network.Body.String())
	}

	withoutGroup, _ := callMonitorHandler(
		t,
		monitorHandler,
		http.MethodPost,
		`{"name":"No Group","monitor_type":"https","target":"https://example.com"}`,
	)
	if withoutGroup != http.StatusCreated {
		t.Fatalf("create without group status = %d", withoutGroup)
	}

	withGroup, _ := callMonitorHandler(
		t,
		monitorHandler,
		http.MethodPost,
		`{"name":"Grouped","monitor_type":"https","target":"https://example.com","group_id":1}`,
	)
	if withGroup != http.StatusCreated {
		t.Fatalf("create with group status = %d", withGroup)
	}

	missingGroup, _ := callMonitorHandler(
		t,
		monitorHandler,
		http.MethodPost,
		`{"name":"Missing","monitor_type":"https","target":"https://example.com","group_id":999}`,
	)
	if missingGroup != http.StatusBadRequest {
		t.Fatalf("create with missing group status = %d", missingGroup)
	}

	invalidGroup, _ := callMonitorHandler(
		t,
		monitorHandler,
		http.MethodPost,
		`{"name":"Invalid","monitor_type":"https","target":"https://example.com","group_id":0}`,
	)
	if invalidGroup != http.StatusBadRequest {
		t.Fatalf("create with invalid group status = %d", invalidGroup)
	}

	statusRecorder := httptest.NewRecorder()
	monitorHandler.Status(
		statusRecorder,
		httptest.NewRequest(http.MethodGet, "/api/v1/monitors/status", nil),
	)
	if statusRecorder.Code != http.StatusOK {
		t.Fatalf("monitor status code = %d", statusRecorder.Code)
	}
	var statuses []service.MonitorStatus
	if err := json.Unmarshal(statusRecorder.Body.Bytes(), &statuses); err != nil {
		t.Fatalf("decode monitor statuses: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("statuses = %+v, want two monitors", statuses)
	}
	if statuses[1].GroupID != nil || statuses[1].GroupName != "" {
		t.Fatalf("ungrouped monitor status = %+v", statuses[1])
	}
	if statuses[0].GroupID == nil || *statuses[0].GroupID != 1 || statuses[0].GroupName != "Production" {
		t.Fatalf("grouped monitor status = %+v", statuses[0])
	}

	updateBody := func(groupJSON string) string {
		return `{"name":"No Group","monitor_type":"https","target":"https://example.com","interval_seconds":60,"timeout_seconds":10,"enabled":true` + groupJSON + `}`
	}

	assigned, _ := callMonitorHandler(t, monitorHandler, http.MethodPut, updateBody(`,"group_id":1`))
	if assigned != http.StatusOK {
		t.Fatalf("assign group status = %d", assigned)
	}

	preserved, _ := callMonitorHandler(t, monitorHandler, http.MethodPut, updateBody(""))
	if preserved != http.StatusOK {
		t.Fatalf("preserve group status = %d", preserved)
	}
	stored, err := store.GetByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("get preserved monitor: %v", err)
	}
	if stored.GroupID == nil || *stored.GroupID != 1 {
		t.Fatalf("omitted group_id did not preserve group: %+v", stored)
	}

	changed, _ := callMonitorHandler(t, monitorHandler, http.MethodPut, updateBody(`,"group_id":2`))
	if changed != http.StatusOK {
		t.Fatalf("change group status = %d", changed)
	}

	removed, _ := callMonitorHandler(t, monitorHandler, http.MethodPut, updateBody(`,"group_id":null`))
	if removed != http.StatusOK {
		t.Fatalf("remove group status = %d", removed)
	}
	stored, err = store.GetByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("get removed monitor: %v", err)
	}
	if stored.GroupID != nil {
		t.Fatalf("explicit null did not remove group: %+v", stored)
	}

	assigned, _ = callMonitorHandler(t, monitorHandler, http.MethodPut, updateBody(`,"group_id":1`))
	if assigned != http.StatusOK {
		t.Fatalf("reassign group status = %d", assigned)
	}

	deleted := callGroupHandler(t, groupHandler, http.MethodDelete, "1", "")
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete group status = %d, body = %s", deleted.Code, deleted.Body.String())
	}

	stored, err = store.GetByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("monitor was deleted with group: %v", err)
	}
	if stored.GroupID != nil || stored.GroupName != "" {
		t.Fatalf("monitor did not become ungrouped: %+v", stored)
	}
}

func TestExistingMonitorTypesAcceptUngroupedCreation(t *testing.T) {
	handler := &MonitorHandler{Repository: newFakeGroupMonitorStore()}

	tests := []struct {
		name string
		body string
	}{
		{"http", `{"name":"HTTP","monitor_type":"http","target":"http://example.com"}`},
		{"https", `{"name":"HTTPS","monitor_type":"https","target":"https://example.com"}`},
		{"tcp", `{"name":"TCP","monitor_type":"tcp","target":"example.com:443"}`},
		{"ssl", `{"name":"SSL","monitor_type":"ssl","target":"example.com"}`},
		{"ping", `{"name":"Ping","monitor_type":"ping","target":"example.com"}`},
		{"dns", `{"name":"DNS","monitor_type":"dns","target":"example.com","config":{"record_type":"A"}}`},
		{"port", `{"name":"Port","monitor_type":"port","target":"example.com","config":{"ports":[443]}}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, body := callMonitorHandler(t, handler, http.MethodPost, test.body)
			if status != http.StatusCreated {
				t.Fatalf("status = %d, body = %s", status, body)
			}
		})
	}
}
