package service

import (
	"context"
	"errors"
	"fmt"
	"hardhw/internal/models"
	"hardhw/internal/storage"
)

type UserService interface {
	CreateUser(ctx context.Context, newUser models.User) (models.User, []models.User, error)
	GetUserByNickname(ctx context.Context, nickname string) (*models.User, error)
	UpdateUser(ctx context.Context, nickname string, updates models.User) (*models.User, error)
}

type userServiceImpl struct {
	userStorage storage.UserStorage
}

func NewUserService(s storage.UserStorage) UserService {
	return &userServiceImpl{userStorage: s}
}

func (s *userServiceImpl) CreateUser(ctx context.Context, newUser models.User) (models.User, []models.User, error) {
	var conflictUsers []models.User
	var createdUser models.User

	foundUserByNickname, err := s.userStorage.GetUserByNickname(ctx, newUser.Nickname)
	if err != nil && !errors.Is(err, models.ErrNotFound) {
		return createdUser, nil, fmt.Errorf("ошибка при поиске пользователя по nickname: %w", err)
	}
	if foundUserByNickname != nil {
		conflictUsers = append(conflictUsers, *foundUserByNickname)
	}

	foundUserByEmail, err := s.userStorage.GetUserByEmail(ctx, newUser.Email)
	if err != nil && !errors.Is(err, models.ErrNotFound) {
		return createdUser, nil, fmt.Errorf("ошибка при поиске пользователя по email: %w", err)
	}
	if foundUserByEmail != nil {

		if foundUserByNickname == nil || foundUserByNickname.Nickname != foundUserByEmail.Nickname {
			conflictUsers = append(conflictUsers, *foundUserByEmail)
		}
	}

	if len(conflictUsers) > 0 {
		return createdUser, conflictUsers, nil
	}

	err = s.userStorage.CreateUser(ctx, &newUser)
	if err != nil {
		if errors.Is(err, models.ErrUserConflict) {

			foundUserByNickname, _ = s.userStorage.GetUserByNickname(ctx, newUser.Nickname)
			foundUserByEmail, _ = s.userStorage.GetUserByEmail(ctx, newUser.Email)

			if foundUserByNickname != nil {
				conflictUsers = append(conflictUsers, *foundUserByNickname)
			}
			if foundUserByEmail != nil {
				if foundUserByNickname == nil || foundUserByNickname.Nickname != foundUserByEmail.Nickname {
					conflictUsers = append(conflictUsers, *foundUserByEmail)
				}
			}
			return createdUser, conflictUsers, models.ErrUserConflict
		}
		return createdUser, nil, fmt.Errorf("ошибка при сохранении пользователя: %w", err)
	}

	return newUser, nil, nil
}

func (s *userServiceImpl) GetUserByNickname(ctx context.Context, nickname string) (*models.User, error) {
	user, err := s.userStorage.GetUserByNickname(ctx, nickname)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("ошибка при получении пользователя по никнейму из хранилища: %w", err)
	}
	return user, nil
}

func (s *userServiceImpl) UpdateUser(ctx context.Context, nickname string, updates models.User) (*models.User, error) {

	existingUser, err := s.userStorage.GetUserByNickname(ctx, nickname)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("ошибка при поиске существующего пользователя для обновления: %w", err)
	}

	if updates.Fullname != "" {
		existingUser.Fullname = updates.Fullname
	}
	if updates.About != "" {
		existingUser.About = updates.About
	}

	if updates.Email != "" {
		if updates.Email != existingUser.Email {

			foundUserByEmail, err := s.userStorage.GetUserByEmail(ctx, updates.Email)

			if err == nil && foundUserByEmail.Nickname != existingUser.Nickname {
				return nil, models.ErrUserConflict
			}

			if err != nil && !errors.Is(err, models.ErrNotFound) {
				return nil, fmt.Errorf("ошибка при проверке конфликта email: %w", err)
			}
		}
		existingUser.Email = updates.Email
	}

	updatedUser, err := s.userStorage.UpdateUser(ctx, *existingUser)
	if err != nil {
		if errors.Is(err, models.ErrUserConflict) {
			return nil, models.ErrUserConflict
		}
		return nil, fmt.Errorf("ошибка при обновлении пользователя в хранилище: %w", err)
	}

	return updatedUser, nil
}
