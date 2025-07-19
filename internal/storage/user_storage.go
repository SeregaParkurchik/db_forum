package storage

import (
	"context"
	"errors"
	"fmt"
	"hardhw/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserStorage interface {
	CreateUser(ctx context.Context, user *models.User) error
	GetUserByNickname(ctx context.Context, nickname string) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	UpdateUser(ctx context.Context, user models.User) (*models.User, error)
}

type postgresUserStorage struct {
	pool *pgxpool.Pool
}

func NewPostgresUserStorage(pool *pgxpool.Pool) UserStorage {
	return &postgresUserStorage{pool: pool}
}

func (p *postgresUserStorage) CreateUser(ctx context.Context, user *models.User) error {
	query := `
        INSERT INTO users (nickname, fullname, email, about)
        VALUES ($1, $2, $3, $4)
        RETURNING nickname -- Возвращаем nickname
    `

	err := p.pool.QueryRow(ctx, query, user.Nickname, user.Fullname, user.Email, user.About).Scan(&user.Nickname)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return models.ErrUserConflict
		}
		return fmt.Errorf("ошибка при создании пользователя: %w", err)
	}

	return nil
}

func (p *postgresUserStorage) GetUserByNickname(ctx context.Context, nickname string) (*models.User, error) {
	var user models.User
	query := `
        SELECT nickname, fullname, email, about
        FROM users
        WHERE nickname = $1
    `
	row := p.pool.QueryRow(ctx, query, nickname)

	err := row.Scan(&user.Nickname, &user.Fullname, &user.Email, &user.About)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("ошибка при запросе пользователя по nickname: %w", err)
	}

	return &user, nil
}

func (p *postgresUserStorage) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	query := `
        SELECT nickname, fullname, email, about
        FROM users
        WHERE email = $1
    `
	row := p.pool.QueryRow(ctx, query, email)

	err := row.Scan(&user.Nickname, &user.Fullname, &user.Email, &user.About)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("ошибка при запросе пользователя по email: %w", err)
	}

	return &user, nil
}

func (p *postgresUserStorage) UpdateUser(ctx context.Context, user models.User) (*models.User, error) {
	query := `
        UPDATE users
        SET fullname = $1, email = $2, about = $3
        WHERE nickname = $4
        RETURNING nickname, fullname, email, about -- Возвращаем без id
    `
	var updatedUser models.User

	err := p.pool.QueryRow(ctx, query, user.Fullname, user.Email, user.About, user.Nickname).
		Scan(&updatedUser.Nickname, &updatedUser.Fullname, &updatedUser.Email, &updatedUser.About)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}

		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, models.ErrUserConflict
		}

		return nil, fmt.Errorf("ошибка при обновлении пользователя в БД: %w", err)
	}

	return &updatedUser, nil
}
