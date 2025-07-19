package models

import (
	"errors"
	"time"
)

type User struct {
	Nickname string `json:"nickname"`
	Fullname string `json:"fullname"`
	About    string `json:"about"`
	Email    string `json:"email"`
}

type Forum struct {
	Title   string `json:"title"`
	User    string `json:"user"`
	Slug    string `json:"slug"`
	Posts   int64  `json:"posts,omitempty"`
	Threads int32  `json:"threads,omitempty"`
}

type Thread struct {
	ID      int64     `json:"id"`
	Title   string    `json:"title"`
	Author  string    `json:"author"`
	Forum   string    `json:"forum"`
	Message string    `json:"message"`
	Votes   int32     `json:"votes"`
	Slug    *string   `json:"slug,omitempty"`
	Created time.Time `json:"created"`
}

type Post struct {
	ID           int64     `json:"id"`
	Parent       int64     `json:"parent"`
	Author       string    `json:"author"`
	Message      string    `json:"message"`
	IsEdited     bool      `json:"isEdited"`
	Forum        string    `json:"forum"`
	Thread       int64     `json:"thread"`
	Created      time.Time `json:"created"`
	Path         []int64   `json:"-"`
	RootParentID int64     `json:"-"`
}

type Vote struct {
	Nickname string `json:"nickname"`
	Voice    int    `json:"voice"`
}

type ThreadUpdate struct {
	Title   *string `json:"title,omitempty"`
	Message *string `json:"message,omitempty"`
}

type PostUpdate struct {
	Message string `json:"message"`
}

type PostDetailsResponse struct {
	Author *User   `json:"author,omitempty"`
	Post   *Post   `json:"post"`
	Thread *Thread `json:"thread,omitempty"`
	Forum  *Forum  `json:"forum,omitempty"`
}

type Status struct {
	User   int `json:"user"`
	Forum  int `json:"forum"`
	Thread int `json:"thread"`
	Post   int `json:"post"`
}

var (
	ErrNotFound      = errors.New("not found")
	ErrOwnerNotFound = errors.New("owner not found")

	ErrUserConflict   = errors.New("user conflict")
	ErrForumConflict  = errors.New("forum conflict")
	ErrThreadConflict = errors.New("thread conflict")

	ErrParentNotFound = errors.New("parent not found")
	ErrPostNotFound   = errors.New("post not found")
)
