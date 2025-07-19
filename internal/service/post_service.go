package service

import (
	"context"
	"errors"
	"fmt"
	"hardhw/internal/models"
	"hardhw/internal/storage"
	"strings"
)

type PostService interface {
	UpdatePostDetails(ctx context.Context, id int64, newMessage string) (*models.Post, error)
	GetPostDetailsWithRelated(ctx context.Context, id int64, related []string) (*models.PostDetailsResponse, error)
	GetPostDetails(ctx context.Context, id int64) (*models.Post, error)
	GetDatabaseStatus(ctx context.Context) (*models.Status, error)
	ClearAllData(ctx context.Context) error
}

type postServiceImpl struct {
	forumStorage  storage.ForumStorage
	userStorage   storage.UserStorage
	threadStorage storage.ThreadStorage
	postStorage   storage.PostStorage
}

func NewPostService(fs storage.ForumStorage, us storage.UserStorage, ts storage.ThreadStorage, ps storage.PostStorage) PostService {
	return &postServiceImpl{forumStorage: fs, userStorage: us, threadStorage: ts, postStorage: ps}
}

func (s *postServiceImpl) GetPostDetails(ctx context.Context, id int64) (*models.Post, error) {
	post, err := s.postStorage.GetPostByID(ctx, id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, models.ErrPostNotFound
		}
		return nil, fmt.Errorf("failed to get post by ID from storage: %w", err)
	}
	return post, nil
}

func (s *postServiceImpl) GetPostDetailsWithRelated(ctx context.Context, id int64, related []string) (*models.PostDetailsResponse, error) {
	post, err := s.postStorage.GetPostByID(ctx, id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, models.ErrPostNotFound
		}
		return nil, fmt.Errorf("failed to get post by ID: %w", err)
	}

	response := &models.PostDetailsResponse{
		Post: post,
	}

	for _, r := range related {
		switch strings.ToLower(r) {
		case "user":
			user, err := s.userStorage.GetUserByNickname(ctx, post.Author)
			if err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return nil, models.ErrNotFound
				}
				return nil, fmt.Errorf("failed to get related user %s: %w", post.Author, err)
			}
			response.Author = user
		case "thread":
			thread, err := s.threadStorage.GetThreadByID(ctx, post.Thread)
			if err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return nil, models.ErrNotFound
				}
				return nil, fmt.Errorf("failed to get related thread %d: %w", post.Thread, err)
			}
			response.Thread = thread
		case "forum":
			forum, err := s.forumStorage.GetForumBySlug(ctx, post.Forum)
			if err != nil {
				if errors.Is(err, models.ErrNotFound) {

					return nil, models.ErrForumConflict
				}
				return nil, fmt.Errorf("failed to get related forum %s: %w", post.Forum, err)
			}
			response.Forum = forum
		default:

			continue
		}
	}

	return response, nil
}

func (s *postServiceImpl) UpdatePostDetails(ctx context.Context, id int64, newMessage string) (*models.Post, error) {
	existingPost, err := s.postStorage.GetPostByID(ctx, id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, models.ErrPostNotFound
		}
		return nil, fmt.Errorf("failed to get existing post for update: %w", err)
	}

	if newMessage == "" {
		return existingPost, nil
	}

	if existingPost.Message == newMessage {
		return existingPost, nil
	}

	updatedPost, err := s.postStorage.UpdatePostMessage(ctx, id, newMessage)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, models.ErrPostNotFound
		}
		return nil, fmt.Errorf("failed to update post message in storage: %w", err)
	}

	return updatedPost, nil
}

func (s *postServiceImpl) GetDatabaseStatus(ctx context.Context) (*models.Status, error) {
	status, err := s.postStorage.CountTableRows(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get database status from storage: %w", err)
	}
	return status, nil
}

func (s *postServiceImpl) ClearAllData(ctx context.Context) error {
	err := s.postStorage.ClearAllTables(ctx)
	if err != nil {
		return fmt.Errorf("failed to clear all data in storage: %w", err)
	}
	return nil
}
