package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/serverwatch/serverwatch/server/internal/service"
)

var (
	ErrGroupNotFound     = errors.New("group not found")
	ErrGroupNameConflict = errors.New("group name already exists")
)

type GroupRepository struct {
	DB *pgxpool.Pool
}

func (r *GroupRepository) CreateGroup(
	ctx context.Context,
	input service.CreateGroupInput,
) (service.Group, error) {
	var group service.Group

	name, err := service.ValidateGroupName(input.Name)
	if err != nil {
		return group, err
	}

	err = r.DB.QueryRow(
		ctx,
		`
		INSERT INTO monitor_groups (name)
		VALUES ($1)
		RETURNING id, name, created_at, updated_at
		`,
		name,
	).Scan(
		&group.ID,
		&group.Name,
		&group.CreatedAt,
		&group.UpdatedAt,
	)
	if isUniqueViolation(err) {
		return service.Group{}, ErrGroupNameConflict
	}

	return group, err
}

func (r *GroupRepository) ListGroups(
	ctx context.Context,
) ([]service.Group, error) {
	rows, err := r.DB.Query(
		ctx,
		`
		SELECT id, name, created_at, updated_at
		FROM monitor_groups
		ORDER BY LOWER(name) ASC, id ASC
		`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groups := make([]service.Group, 0)

	for rows.Next() {
		var group service.Group
		if err := rows.Scan(
			&group.ID,
			&group.Name,
			&group.CreatedAt,
			&group.UpdatedAt,
		); err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}

	return groups, rows.Err()
}

func (r *GroupRepository) UpdateGroup(
	ctx context.Context,
	id int64,
	input service.UpdateGroupInput,
) (service.Group, error) {
	var group service.Group

	name, err := service.ValidateGroupName(input.Name)
	if err != nil {
		return group, err
	}

	err = r.DB.QueryRow(
		ctx,
		`
		UPDATE monitor_groups
		SET name = $2, updated_at = NOW()
		WHERE id = $1
		RETURNING id, name, created_at, updated_at
		`,
		id,
		name,
	).Scan(
		&group.ID,
		&group.Name,
		&group.CreatedAt,
		&group.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return service.Group{}, ErrGroupNotFound
	}
	if isUniqueViolation(err) {
		return service.Group{}, ErrGroupNameConflict
	}

	return group, err
}

func (r *GroupRepository) DeleteGroup(
	ctx context.Context,
	id int64,
) error {
	tag, err := r.DB.Exec(
		ctx,
		`DELETE FROM monitor_groups WHERE id = $1`,
		id,
	)
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return ErrGroupNotFound
	}

	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}
