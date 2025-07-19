package storage

import (
	"context"
	"errors"
	"fmt"
	"hardhw/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostStorage interface {
	GetPostByID(ctx context.Context, id int64) (*models.Post, error)
	UpdatePostMessage(ctx context.Context, id int64, newMessage string) (*models.Post, error)
	CountTableRows(ctx context.Context) (*models.Status, error)
	ClearAllTables(ctx context.Context) error
}

type postgresPostStorage struct {
	pool *pgxpool.Pool
}

func NewPostgresPostStorage(pool *pgxpool.Pool) PostStorage {
	return &postgresPostStorage{pool: pool}
}

func (s *postgresPostStorage) GetPostByID(ctx context.Context, id int64) (*models.Post, error) {
	query := `
		SELECT id, parent, author, message, is_edited, forum, thread_id, created -- ИСПРАВЛЕНО: forum_slug на forum
		FROM posts
		WHERE id = $1
	`
	post := &models.Post{}
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&post.ID,
		&post.Parent,
		&post.Author,
		&post.Message,
		&post.IsEdited,
		&post.Forum,
		&post.Thread,
		&post.Created,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get post by ID: %w", err)
	}
	return post, nil
}

func (s *postgresPostStorage) UpdatePostMessage(ctx context.Context, id int64, newMessage string) (*models.Post, error) {
	query := `
		UPDATE posts
		SET message = $1, is_edited = TRUE
		WHERE id = $2
		RETURNING id, parent, author, message, is_edited, forum, thread_id, created -- ИСПРАВЛЕНО: forum_slug на forum
	`
	updatedPost := &models.Post{}
	err := s.pool.QueryRow(ctx, query, newMessage, id).Scan(
		&updatedPost.ID,
		&updatedPost.Parent,
		&updatedPost.Author,
		&updatedPost.Message,
		&updatedPost.IsEdited,
		&updatedPost.Forum,
		&updatedPost.Thread,
		&updatedPost.Created,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("failed to update post message: %w", err)
	}
	return updatedPost, nil
}

func (s *postgresPostStorage) CountTableRows(ctx context.Context) (*models.Status, error) {
	status := &models.Status{}

	err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM forums").Scan(&status.Forum)
	if err != nil {
		return nil, fmt.Errorf("failed to count forums: %w", err)
	}
	err = s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM posts").Scan(&status.Post)
	if err != nil {
		return nil, fmt.Errorf("failed to count posts: %w", err)
	}
	err = s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM threads").Scan(&status.Thread)
	if err != nil {
		return nil, fmt.Errorf("failed to count threads: %w", err)
	}
	err = s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&status.User)
	if err != nil {
		return nil, fmt.Errorf("failed to count users: %w", err)
	}

	return status, nil
}

func (s *postgresPostStorage) ClearAllTables(ctx context.Context) error {
	query := `
		TRUNCATE TABLE users RESTART IDENTITY CASCADE;
		TRUNCATE TABLE forums RESTART IDENTITY CASCADE;
		TRUNCATE TABLE threads RESTART IDENTITY CASCADE;
		TRUNCATE TABLE posts RESTART IDENTITY CASCADE;
		TRUNCATE TABLE votes RESTART IDENTITY CASCADE;
	`
	_, err := s.pool.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to truncate tables: %w", err)
	}
	return nil
}
