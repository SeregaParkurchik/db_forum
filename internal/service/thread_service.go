package service

import (
	"context"
	"errors"
	"fmt"
	"hardhw/internal/models"
	"hardhw/internal/storage"
	"log"
	"time"
)

type ThreadService interface {
	CreatePosts(ctx context.Context, slugOrID string, newPosts []models.Post) ([]models.Post, error)
	VoteThread(ctx context.Context, slugOrID string, vote models.Vote) (models.Thread, error)
	GetThreadDetails(ctx context.Context, slugOrID string) (models.Thread, error)
	GetThreadPosts(ctx context.Context, slugOrID string, limit int, since int64, sort string, desc bool) ([]models.Post, error)
	UpdateThread(ctx context.Context, slugOrID string, updateData models.ThreadUpdate) (models.Thread, error)
}

type threadServiceImpl struct {
	forumStorage  storage.ForumStorage
	userStorage   storage.UserStorage
	threadStorage storage.ThreadStorage
}

func NewThreadService(fs storage.ForumStorage, us storage.UserStorage, ts storage.ThreadStorage) ThreadService {
	return &threadServiceImpl{forumStorage: fs, userStorage: us, threadStorage: ts}
}

func (s *threadServiceImpl) CreatePosts(ctx context.Context, slugOrID string, newPosts []models.Post) ([]models.Post, error) {
	threadID, err := s.threadStorage.GetThreadIDBySlugOrID(ctx, slugOrID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get thread ID: %w", err)
	}

	if len(newPosts) == 0 {
		return []models.Post{}, nil
	}

	uniqueAuthors := make(map[string]struct{})
	for _, post := range newPosts {
		uniqueAuthors[post.Author] = struct{}{}
	}

	for author := range uniqueAuthors {
		_, err := s.userStorage.GetUserByNickname(ctx, author)
		if err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return nil, models.ErrOwnerNotFound
			}
			return nil, fmt.Errorf("failed to check post author '%s': %w", author, err)
		}
	}

	for _, post := range newPosts {
		if post.Parent != 0 {
			parentPostExists, err := s.threadStorage.CheckParentPostExistsInThread(ctx, post.Parent, threadID)
			if err != nil {
				return nil, fmt.Errorf("failed to check parent post existence for post %d: %w", post.Parent, err)
			}
			if !parentPostExists {
				return nil, models.ErrParentNotFound
			}
		}
	}

	creationTime := time.Now()

	postsToCreate := make([]*models.Post, len(newPosts))
	for i, post := range newPosts {
		postsToCreate[i] = &models.Post{
			Parent:  post.Parent,
			Author:  post.Author,
			Message: post.Message,

			Forum:   "",
			Thread:  threadID,
			Created: creationTime,
		}
	}

	createdPosts, err := s.threadStorage.CreatePosts(ctx, postsToCreate)
	if err != nil {
		return nil, fmt.Errorf("failed to create posts in storage: %w", err)
	}

	return createdPosts, nil
}

func (s *threadServiceImpl) VoteThread(ctx context.Context, slugOrID string, vote models.Vote) (models.Thread, error) {

	_, err := s.userStorage.GetUserByNickname(ctx, vote.Nickname)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return models.Thread{}, models.ErrOwnerNotFound
		}
		return models.Thread{}, fmt.Errorf("failed to check voter existence: %w", err)
	}

	threadID, err := s.threadStorage.GetThreadIDBySlugOrID(ctx, slugOrID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return models.Thread{}, models.ErrNotFound
		}
		return models.Thread{}, fmt.Errorf("failed to get thread ID for voting: %w", err)
	}

	updatedThread, err := s.threadStorage.UpdateThreadVote(ctx, threadID, vote.Nickname, vote.Voice)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return models.Thread{}, models.ErrNotFound
		}
		return models.Thread{}, fmt.Errorf("failed to update thread vote: %w", err)
	}

	return *updatedThread, nil
}

func (s *threadServiceImpl) GetThreadDetails(ctx context.Context, slugOrID string) (models.Thread, error) {
	thread, err := s.threadStorage.GetThreadBySlugOrID(ctx, slugOrID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return models.Thread{}, models.ErrNotFound
		}
		return models.Thread{}, fmt.Errorf("failed to get thread details from storage: %w", err)
	}
	return *thread, nil
}

func (s *threadServiceImpl) GetThreadPosts(ctx context.Context, slugOrID string, limit int, since int64, sort string, desc bool) ([]models.Post, error) {
	threadID, err := s.threadStorage.GetThreadIDBySlugOrID(ctx, slugOrID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get thread ID for posts: %w", err)
	}

	var posts []models.Post
	switch sort {
	case "flat":
		posts, err = s.threadStorage.GetFlatThreadPosts(ctx, threadID, limit, since, desc)
	case "tree":
		posts, err = s.threadStorage.GetTreeThreadPosts(ctx, threadID, limit, since, desc)
	case "parent_tree":
		posts, err = s.threadStorage.GetParentTreeThreadPosts(ctx, threadID, limit, since, desc)
	default:
		return nil, errors.New("invalid sort type")
	}

	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return []models.Post{}, nil
		}

		return nil, fmt.Errorf("failed to get thread posts from storage for sort %s: %w", sort, err)
	}

	log.Printf("DEBUG: GetThreadPosts for thread %d, sort %s, desc %t, limit %d, since %d - returning %d posts. Is nil: %t",
		threadID, sort, desc, limit, since, len(posts), posts == nil)
	return posts, nil
}

func (s *threadServiceImpl) UpdateThread(ctx context.Context, slugOrID string, updateData models.ThreadUpdate) (models.Thread, error) {
	updatedThread, err := s.threadStorage.UpdateThread(ctx, slugOrID, updateData)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return models.Thread{}, models.ErrNotFound
		}
		return models.Thread{}, fmt.Errorf("failed to update thread: %w", err)
	}

	return updatedThread, nil
}
