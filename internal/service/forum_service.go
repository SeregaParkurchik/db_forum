package service

import (
	"context"
	"errors"
	"fmt"
	"hardhw/internal/models"
	"hardhw/internal/storage"
	"time"
)

type ForumService interface {
	CreateForum(ctx context.Context, newForum models.Forum) (models.Forum, error)
	GetForumBySlug(ctx context.Context, slug string) (*models.Forum, error)
	CreateThread(ctx context.Context, forumSlug string, newThread models.Thread) (models.Thread, error)
	GetForumThreads(ctx context.Context, forumSlug string, limit int, since *time.Time, desc bool) ([]models.Thread, error)
	GetForumUsers(ctx context.Context, slug string, limit int, since string, desc bool) ([]models.User, error)
}

type forumServiceImpl struct {
	forumStorage storage.ForumStorage
	userStorage  storage.UserStorage
}

func NewForumService(fs storage.ForumStorage, us storage.UserStorage) ForumService {
	return &forumServiceImpl{forumStorage: fs, userStorage: us}
}

func (s *forumServiceImpl) CreateForum(ctx context.Context, newForum models.Forum) (models.Forum, error) {
	userFromDB, err := s.userStorage.GetUserByNickname(ctx, newForum.User)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return models.Forum{}, models.ErrOwnerNotFound
		}
		return models.Forum{}, fmt.Errorf("failed to check user existence: %w", err)
	}

	existingForum, err := s.forumStorage.GetForumBySlug(ctx, newForum.Slug)
	if err == nil {
		existingForum.User = userFromDB.Nickname
		return *existingForum, models.ErrForumConflict
	}

	if !errors.Is(err, models.ErrNotFound) {
		return models.Forum{}, fmt.Errorf("failed to check forum existence: %w", err)
	}

	forumToCreate := &models.Forum{
		Slug:  newForum.Slug,
		Title: newForum.Title,
		User:  userFromDB.Nickname,
	}

	createdForum, err := s.forumStorage.CreateForum(ctx, forumToCreate)
	if err != nil {
		if errors.Is(err, models.ErrForumConflict) {
			recheckForum, recheckErr := s.forumStorage.GetForumBySlug(ctx, newForum.Slug)
			if recheckErr == nil {
				recheckForum.User = userFromDB.Nickname
				return *recheckForum, models.ErrForumConflict
			}
			return models.Forum{}, fmt.Errorf("failed to retrieve conflicting forum after creation attempt: %w", err)
		}
		return models.Forum{}, fmt.Errorf("failed to save new forum: %w", err)
	}

	createdForum.User = userFromDB.Nickname

	return *createdForum, nil
}

func (s *forumServiceImpl) GetForumBySlug(ctx context.Context, slug string) (*models.Forum, error) {
	forum, err := s.forumStorage.GetForumBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, models.ErrNotFound
		}

		return nil, fmt.Errorf("ошибка при получении форума по slug из хранилища: %w", err)
	}
	return forum, nil
}

func (s *forumServiceImpl) CreateThread(ctx context.Context, forumSlug string, newThread models.Thread) (models.Thread, error) {
	forumFromDB, err := s.forumStorage.GetForumBySlug(ctx, forumSlug)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return models.Thread{}, models.ErrNotFound
		}
		return models.Thread{}, fmt.Errorf("failed to check forum existence: %w", err)
	}

	userFromDB, err := s.userStorage.GetUserByNickname(ctx, newThread.Author)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return models.Thread{}, models.ErrOwnerNotFound
		}
		return models.Thread{}, fmt.Errorf("failed to check author existence: %w", err)
	}

	if newThread.Slug != nil && *newThread.Slug != "" {
		existingThread, err := s.forumStorage.GetThreadBySlug(ctx, *newThread.Slug)
		if err == nil {
			return *existingThread, models.ErrThreadConflict
		}
		if !errors.Is(err, models.ErrNotFound) {
			return models.Thread{}, fmt.Errorf("failed to check thread existence by slug: %w", err)
		}
	}

	creationTime := newThread.Created
	if creationTime.IsZero() {
		creationTime = time.Now()
	}

	threadToCreate := &models.Thread{
		Title:   newThread.Title,
		Author:  userFromDB.Nickname,
		Forum:   forumFromDB.Slug,
		Message: newThread.Message,
		Slug:    newThread.Slug,
		Created: creationTime,
		Votes:   0,
	}

	createdThread, err := s.forumStorage.CreateThread(ctx, threadToCreate)
	if err != nil {
		if errors.Is(err, models.ErrThreadConflict) {
			if threadToCreate.Slug != nil && *threadToCreate.Slug != "" {
				recheckThread, recheckErr := s.forumStorage.GetThreadBySlug(ctx, *threadToCreate.Slug)
				if recheckErr == nil {
					return *recheckThread, models.ErrThreadConflict
				}
			}
			return models.Thread{}, fmt.Errorf("failed to retrieve conflicting thread after creation attempt: %w", err)
		}
		return models.Thread{}, fmt.Errorf("failed to save new thread: %w", err)
	}

	err = s.forumStorage.IncrementForumThreadsCount(ctx, forumFromDB.Slug)
	if err != nil {
		fmt.Printf("Warning: Could not increment thread count for forum %s: %v\n", forumFromDB.Slug, err)
	}

	return *createdThread, nil
}

func (s *forumServiceImpl) GetForumThreads(ctx context.Context, forumSlug string, limit int, since *time.Time, desc bool) ([]models.Thread, error) {

	_, err := s.forumStorage.GetForumBySlug(ctx, forumSlug)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("ошибка при проверке существования форума '%s': %w", forumSlug, err)
	}

	threads, err := s.forumStorage.GetThreadsByForumSlug(ctx, forumSlug, limit, since, desc) // <-- Предполагается этот метод в ForumStorage
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {

			return []models.Thread{}, nil
		}
		return nil, fmt.Errorf("ошибка при получении веток для форума '%s' из хранилища: %w", forumSlug, err)
	}

	return threads, nil
}

func (s *forumServiceImpl) GetForumUsers(ctx context.Context, slug string, limit int, since string, desc bool) ([]models.User, error) {
	_, err := s.forumStorage.GetForumBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("failed to check forum existence: %w", err)
	}

	users, err := s.forumStorage.GetForumUsers(ctx, slug, limit, since, desc)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve forum users from storage: %w", err)
	}

	return users, nil
}
