package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/serverwatch/serverwatch/server/internal/repository"
	"github.com/serverwatch/serverwatch/server/internal/service"
)

type GroupStore interface {
	CreateGroup(context.Context, service.CreateGroupInput) (service.Group, error)
	ListGroups(context.Context) ([]service.Group, error)
	UpdateGroup(context.Context, int64, service.UpdateGroupInput) (service.Group, error)
	DeleteGroup(context.Context, int64) error
}

type GroupHandler struct {
	Repository GroupStore
}

func (h *GroupHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input service.CreateGroupInput

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "invalid_json",
		})
		return
	}

	name, err := service.ValidateGroupName(input.Name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": err.Error(),
		})
		return
	}
	input.Name = name

	group, err := h.Repository.CreateGroup(r.Context(), input)
	if errors.Is(err, repository.ErrGroupNameConflict) {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error": "group name already exists",
		})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "failed to create group",
		})
		return
	}

	writeJSON(w, http.StatusCreated, group)
}

func (h *GroupHandler) List(w http.ResponseWriter, r *http.Request) {
	groups, err := h.Repository.ListGroups(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "failed to list groups",
		})
		return
	}

	writeJSON(w, http.StatusOK, groups)
}

func (h *GroupHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseGroupID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "invalid group id",
		})
		return
	}

	var input service.UpdateGroupInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "invalid_json",
		})
		return
	}

	name, err := service.ValidateGroupName(input.Name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": err.Error(),
		})
		return
	}
	input.Name = name

	group, err := h.Repository.UpdateGroup(r.Context(), id, input)
	if errors.Is(err, repository.ErrGroupNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error": "group not found",
		})
		return
	}
	if errors.Is(err, repository.ErrGroupNameConflict) {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error": "group name already exists",
		})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "failed to update group",
		})
		return
	}

	writeJSON(w, http.StatusOK, group)
}

func (h *GroupHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseGroupID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "invalid group id",
		})
		return
	}

	err = h.Repository.DeleteGroup(r.Context(), id)
	if errors.Is(err, repository.ErrGroupNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error": "group not found",
		})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "failed to delete group",
		})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseGroupID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid group id")
	}
	return id, nil
}
