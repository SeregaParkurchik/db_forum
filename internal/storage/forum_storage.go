package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"hardhw/internal/models"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ForumStorage interface {
	GetForumBySlug(ctx context.Context, slug string) (*models.Forum, error)
	CreateForum(ctx context.Context, forum *models.Forum) (*models.Forum, error)
	IncrementForumThreadsCount(ctx context.Context, forumSlug string) error
	GetThreadBySlug(ctx context.Context, slug string) (*models.Thread, error)
	CreateThread(ctx context.Context, thread *models.Thread) (*models.Thread, error)
	GetThreadByID(ctx context.Context, id uuid.UUID) (*models.Thread, error)
	GetThreadsByForumSlug(ctx context.Context, forumSlug string, limit int, since *time.Time, desc bool) ([]models.Thread, error)
	GetForumUsers(ctx context.Context, slug string, limit int, since string, desc bool) ([]models.User, error)
}

type postgresForumStorage struct {
	pool *pgxpool.Pool
}

func NewPostgresForumStorage(pool *pgxpool.Pool) ForumStorage {
	return &postgresForumStorage{pool: pool}
}

func (s *postgresForumStorage) GetForumBySlug(ctx context.Context, slug string) (*models.Forum, error) {
	query := `
        SELECT slug, title, user_nickname, posts, threads -- Удален id из SELECT
        FROM forums
        WHERE slug = $1`

	forum := &models.Forum{}
	err := s.pool.QueryRow(ctx, query, slug).Scan(
		&forum.Slug,
		&forum.Title,
		&forum.User,
		&forum.Posts,
		&forum.Threads,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get forum by slug: %w", err)
	}

	return forum, nil
}

func (s *postgresForumStorage) CreateForum(ctx context.Context, forum *models.Forum) (*models.Forum, error) {

	var canonicalUserNickname string
	getUserQuery := `SELECT nickname FROM users WHERE nickname = $1`
	err := s.pool.QueryRow(ctx, getUserQuery, forum.User).Scan(&canonicalUserNickname)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get canonical user nickname for forum creator: %w", err)
	}

	query := `
        INSERT INTO forums (slug, title, user_nickname)
        VALUES ($1, $2, $3)
        RETURNING slug, title, user_nickname, posts, threads`

	var createdForum models.Forum
	err = s.pool.QueryRow(ctx, query, forum.Slug, forum.Title, canonicalUserNickname).Scan( // Используем canonicalUserNickname
		&createdForum.Slug,
		&createdForum.Title,
		&createdForum.User,
		&createdForum.Posts,
		&createdForum.Threads,
	)
	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, models.ErrForumConflict
		}

		return nil, fmt.Errorf("failed to insert forum: %w", err)
	}

	return &createdForum, nil
}

func (s *postgresForumStorage) EnsureForumUserExists(ctx context.Context, forumSlug, userNickname string) error {
	query := `
        INSERT INTO forum_users (forum_slug, user_nickname)
        VALUES ($1, $2)
        ON CONFLICT (forum_slug, user_nickname) DO NOTHING`

	_, err := s.pool.Exec(ctx, query, forumSlug, userNickname)
	if err != nil {
		return fmt.Errorf("failed to insert forum user %s for forum %s: %w", userNickname, forumSlug, err)
	}
	return nil
}

func (s *postgresForumStorage) IncrementForumThreadsCount(ctx context.Context, forumSlug string) error {
	query := `UPDATE forums SET threads = threads + 1 WHERE slug = $1`
	commandTag, err := s.pool.Exec(ctx, query, forumSlug)
	if err != nil {
		return fmt.Errorf("failed to increment forum threads count for slug %s: %w", forumSlug, err)
	}
	if commandTag.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}

func (s *postgresForumStorage) GetThreadBySlug(ctx context.Context, slug string) (*models.Thread, error) {
	query := `
        SELECT id, title, author, forum, message, votes, slug, created
        FROM threads
        WHERE slug = $1`

	var thread models.Thread
	var nullSlug sql.NullString
	err := s.pool.QueryRow(ctx, query, slug).Scan(
		&thread.ID,
		&thread.Title,
		&thread.Author,
		&thread.Forum,
		&thread.Message,
		&thread.Votes,
		&nullSlug,
		&thread.Created,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get thread by slug %s: %w", slug, err)
	}

	if nullSlug.Valid {
		thread.Slug = &nullSlug.String
	} else {
		thread.Slug = nil
	}

	return &thread, nil
}

func (s *postgresForumStorage) CreateThread(ctx context.Context, thread *models.Thread) (*models.Thread, error) {

	var canonicalAuthorNickname string
	getUserQuery := `SELECT nickname FROM users WHERE nickname = $1`
	err := s.pool.QueryRow(ctx, getUserQuery, thread.Author).Scan(&canonicalAuthorNickname)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get canonical author nickname: %w", err)
	}

	var forumSlugCheck string
	checkForumQuery := `SELECT slug FROM forums WHERE slug = $1`
	err = s.pool.QueryRow(ctx, checkForumQuery, thread.Forum).Scan(&forumSlugCheck)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("failed to check forum existence: %w", err)
	}

	query := `
        INSERT INTO threads (title, author, forum, message, slug, created)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING id, title, author, forum, message, votes, slug, created`

	var slugSQL sql.NullString
	if thread.Slug != nil && *thread.Slug != "" {
		slugSQL.String = *thread.Slug
		slugSQL.Valid = true
	}

	createdTime := time.Now()
	if !thread.Created.IsZero() {
		createdTime = thread.Created
	}

	var created models.Thread
	var scannedSlug sql.NullString
	err = s.pool.QueryRow(ctx, query,
		thread.Title,
		canonicalAuthorNickname,
		thread.Forum,
		thread.Message,
		slugSQL,
		createdTime,
	).Scan(
		&created.ID,
		&created.Title,
		&created.Author,
		&created.Forum,
		&created.Message,
		&created.Votes,
		&scannedSlug,
		&created.Created,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" {
				return nil, models.ErrThreadConflict
			}

			if pgErr.Code == "23503" {
				return nil, models.ErrNotFound
			}
		}
		return nil, fmt.Errorf("failed to insert thread: %w", err)
	}

	if scannedSlug.Valid {
		created.Slug = &scannedSlug.String
	} else {
		created.Slug = nil
	}

	err = s.EnsureForumUserExists(ctx, created.Forum, created.Author)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure forum user exists for thread creator %s in forum %s: %w", created.Author, created.Forum, err)
	}

	return &created, nil
}

func (s *postgresForumStorage) GetThreadByID(ctx context.Context, id uuid.UUID) (*models.Thread, error) {
	query := `
        SELECT id, title, author, forum, message, votes, slug, created
        FROM threads
        WHERE id = $1`

	var thread models.Thread
	var nullSlug sql.NullString
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&thread.ID,
		&thread.Title,
		&thread.Author,
		&thread.Forum,
		&thread.Message,
		&thread.Votes,
		&nullSlug,
		&thread.Created,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get thread by id %s: %w", id.String(), err)
	}

	if nullSlug.Valid {
		thread.Slug = &nullSlug.String
	} else {
		thread.Slug = nil
	}

	return &thread, nil
}

func (s *postgresForumStorage) GetThreadsByForumSlug(ctx context.Context, forumSlug string, limit int, since *time.Time, desc bool) ([]models.Thread, error) {
	var (
		queryBuilder strings.Builder
		args         []interface{}
		argCount     int
	)

	queryBuilder.WriteString(`
        SELECT id, title, author, forum, message, votes, slug, created
        FROM threads
        WHERE forum = $1`)
	args = append(args, forumSlug)
	argCount = 1

	if since != nil {
		argCount++
		if desc {
			queryBuilder.WriteString(fmt.Sprintf(" AND created <= $%d", argCount))
		} else {
			queryBuilder.WriteString(fmt.Sprintf(" AND created >= $%d", argCount))
		}
		args = append(args, *since)
	}

	if desc {
		queryBuilder.WriteString(" ORDER BY created DESC")
	} else {
		queryBuilder.WriteString(" ORDER BY created ASC")
	}

	argCount++
	queryBuilder.WriteString(fmt.Sprintf(" LIMIT $%d", argCount))
	args = append(args, limit)

	rows, err := s.pool.Query(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query threads for forum %s: %w", forumSlug, err)
	}
	defer rows.Close()

	var threads []models.Thread
	for rows.Next() {
		var thread models.Thread
		var nullSlug sql.NullString

		err := rows.Scan(
			&thread.ID,
			&thread.Title,
			&thread.Author,
			&thread.Forum,
			&thread.Message,
			&thread.Votes,
			&nullSlug,
			&thread.Created,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan thread row for forum %s: %w", forumSlug, err)
		}

		if nullSlug.Valid {
			thread.Slug = &nullSlug.String
		} else {
			thread.Slug = nil
		}
		threads = append(threads, thread)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over thread rows for forum %s: %w", forumSlug, err)
	}

	if len(threads) == 0 {
		return []models.Thread{}, nil
	}

	return threads, nil
}

func (s *postgresForumStorage) GetForumUsers(ctx context.Context, slug string, limit int, since string, desc bool) ([]models.User, error) {
	users := make([]models.User, 0)

	baseQuery := `
        SELECT
            u.nickname, u.fullname, u.about, u.email
        FROM users u
        JOIN forum_users fu ON u.nickname = fu.user_nickname
        WHERE fu.forum_slug = $1
    `

	args := []interface{}{slug}
	paramCounter := 2

	if since != "" {
		if desc {
			baseQuery += fmt.Sprintf(" AND lower(u.nickname) COLLATE \"C\" < lower($%d) COLLATE \"C\" ", paramCounter)
		} else {
			baseQuery += fmt.Sprintf(" AND lower(u.nickname) COLLATE \"C\" > lower($%d) COLLATE \"C\" ", paramCounter)
		}
		args = append(args, since)
		paramCounter++
	}

	if desc {
		baseQuery += " ORDER BY lower(u.nickname) COLLATE \"C\" DESC, u.nickname COLLATE \"C\" DESC "
	} else {
		baseQuery += " ORDER BY lower(u.nickname) COLLATE \"C\" ASC, u.nickname COLLATE \"C\" ASC "
	}

	baseQuery += fmt.Sprintf(" LIMIT $%d", paramCounter)
	args = append(args, limit)

	rows, err := s.pool.Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query forum users from forum_users table: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var user models.User
		err := rows.Scan(
			&user.Nickname,
			&user.Fullname,
			&user.About,
			&user.Email,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan forum user: %w", err)
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return users, nil
}
