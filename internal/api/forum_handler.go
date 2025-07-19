package api

import (
	"errors"
	"fmt"
	"hardhw/internal/models"
	"hardhw/internal/service"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type ForumHandler struct {
	forumService service.ForumService
}

func NewForumHandler(s service.ForumService) *ForumHandler {
	return &ForumHandler{forumService: s}
}

func (h *ForumHandler) CreateForum(c *gin.Context) {
	var newForum models.Forum

	if err := c.ShouldBindJSON(&newForum); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body"})
		return
	}

	createdForum, err := h.forumService.CreateForum(c.Request.Context(), newForum)

	if err != nil {
		switch err {
		case models.ErrForumConflict:
			c.JSON(http.StatusConflict, createdForum)
			return
		case models.ErrOwnerNotFound:
			c.JSON(http.StatusNotFound, gin.H{"message": "Can't find user with id #%s" /* + newForum.User*/})
			return
		default:
			log.Printf("Error creating forum: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
			return
		}
	}

	c.JSON(http.StatusCreated, createdForum)
}

func (h *ForumHandler) GetForumDetails(c *gin.Context) {
	slug := c.Param("slug")

	forum, err := h.forumService.GetForumBySlug(c.Request.Context(), slug)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {

			c.JSON(http.StatusNotFound, gin.H{"message": fmt.Sprintf("Can't find forum with slug: %s", slug)})
			return
		}

		log.Printf("Error getting forum details: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, forum)
}

func (h *ForumHandler) CreateThread(c *gin.Context) {
	forumSlug := c.Param("slug")

	var newThread models.Thread
	if err := c.ShouldBindJSON(&newThread); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body"})
		return
	}

	createdThread, err := h.forumService.CreateThread(c.Request.Context(), forumSlug, newThread)

	if err != nil {
		switch err {
		case models.ErrNotFound:
			c.JSON(http.StatusNotFound, gin.H{"message": "Can't find forum with slug " + forumSlug})
			return
		case models.ErrOwnerNotFound:
			c.JSON(http.StatusNotFound, gin.H{"message": "Can't find user with nickname " + newThread.Author})
			return
		case models.ErrThreadConflict:
			c.JSON(http.StatusConflict, createdThread)
			return
		default:
			log.Printf("Error creating thread: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
			return
		}
	}

	c.JSON(http.StatusCreated, createdThread)
}

func (h *ForumHandler) GetForumThreads(c *gin.Context) {
	forumSlug := c.Param("slug")

	limit := 100
	if limitStr := c.Query("limit"); limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid 'limit' parameter"})
			return
		}
		limit = parsedLimit
	}

	var since *time.Time
	if sinceStr := c.Query("since"); sinceStr != "" {
		parsedTime, err := time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid 'since' parameter format"})
			return
		}
		since = &parsedTime
	}

	desc := false
	if descStr := c.Query("desc"); descStr != "" {
		parsedDesc, err := strconv.ParseBool(descStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid 'desc' parameter"})
			return
		}
		desc = parsedDesc
	}

	threads, err := h.forumService.GetForumThreads(c.Request.Context(), forumSlug, limit, since, desc)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {

			c.JSON(http.StatusNotFound, gin.H{"message": fmt.Sprintf("Can't find forum by slug: %s", forumSlug)})
			return
		}

		log.Printf("Error getting forum threads: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, threads)
}

func (h *ForumHandler) GetForumUsers(c *gin.Context) {
	slug := c.Param("slug")

	limitStr := c.DefaultQuery("limit", "100")
	since := c.Query("since")
	descStr := c.DefaultQuery("desc", "false")

	limit, err := strconv.ParseInt(limitStr, 10, 32)
	if err != nil || limit < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid limit parameter"})
		return
	}

	desc := false
	if descStr == "true" {
		desc = true
	} else if descStr != "false" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid desc parameter, must be 'true' or 'false'"})
		return
	}

	users, err := h.forumService.GetForumUsers(c.Request.Context(), slug, int(limit), since, desc)

	if err != nil {
		switch err {
		case models.ErrNotFound:
			c.JSON(http.StatusNotFound, gin.H{"message": fmt.Sprintf("Can't find forum with slug: %s\n", slug)})
			return
		default:
			log.Printf("Error getting forum users for slug %s: %v", slug, err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
			return
		}
	}

	c.JSON(http.StatusOK, users)
}
