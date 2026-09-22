package service

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const MaxGroupNameLength = 100

var (
	ErrGroupNameRequired = errors.New("group name is required")
	ErrGroupNameTooLong  = errors.New("group name must be at most 100 characters")
)

type Group struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateGroupInput struct {
	Name string `json:"name"`
}

type UpdateGroupInput struct {
	Name string `json:"name"`
}

// ValidateGroupName trims user input and enforces the shared group-name rules
// before a name reaches the database.
func ValidateGroupName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrGroupNameRequired
	}
	if utf8.RuneCountInString(name) > MaxGroupNameLength {
		return "", ErrGroupNameTooLong
	}
	return name, nil
}
