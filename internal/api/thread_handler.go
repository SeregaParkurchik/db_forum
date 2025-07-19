package api

import (
	"hardhw/internal/models"
	"hardhw/internal/service"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type ThreadHandler struct {
	threadService service.ThreadService
}

func NewThreadHandler(s service.ThreadService) *ThreadHandler {
	return &ThreadHandler{threadService: s}
}

func (h *ThreadHandler) CreatePosts(c *gin.Context) {

	slugOrID := c.Param("slug_or_id")

	var newPosts []models.Post
	if err := c.ShouldBindJSON(&newPosts); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body"})
		return
	}

	createdPosts, err := h.threadService.CreatePosts(c.Request.Context(), slugOrID, newPosts)

	if err != nil {
		switch err {
		case models.ErrNotFound:
			c.JSON(http.StatusNotFound, gin.H{"message": "Can't find thread with slug or id: " + slugOrID})
			return
		case models.ErrParentNotFound:
			c.JSON(http.StatusConflict, gin.H{"message": "Parent post not found or not in this thread"})
			return
		case models.ErrOwnerNotFound:
			c.JSON(http.StatusNotFound, gin.H{"message": "One or more post authors not found"})
			return
		default:
			log.Printf("Error creating posts for thread %s: %v", slugOrID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
			return
		}
	}

	c.JSON(http.StatusCreated, createdPosts)
}

func (h *ThreadHandler) VoteThread(c *gin.Context) {
	slugOrID := c.Param("slug_or_id")
	var vote models.Vote

	if err := c.ShouldBindJSON(&vote); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body"})
		return
	}

	updatedThread, err := h.threadService.VoteThread(c.Request.Context(), slugOrID, vote)
	if err != nil {
		switch err {
		case models.ErrNotFound:
			c.JSON(http.StatusNotFound, gin.H{"message": "Can't find thread with slug or id: " + slugOrID})
		case models.ErrOwnerNotFound:
			c.JSON(http.StatusNotFound, gin.H{"message": "Can't find user with nickname: " + vote.Nickname})
		default:
			log.Printf("Error voting for thread %s: %v", slugOrID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, updatedThread)
}

func (h *ThreadHandler) GetThreadDetails(c *gin.Context) {
	slugOrID := c.Param("slug_or_id")

	thread, err := h.threadService.GetThreadDetails(c.Request.Context(), slugOrID)
	if err != nil {
		if err == models.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"message": "Can't find thread with slug or id: " + slugOrID})
			return
		}
		log.Printf("Error getting thread details for %s: %v", slugOrID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, thread)
}

func (h *ThreadHandler) GetThreadPosts(c *gin.Context) {
	slugOrID := c.Param("slug_or_id")

	limitStr := c.Query("limit")
	sinceStr := c.Query("since")
	sort := c.DefaultQuery("sort", "flat")
	descStr := c.DefaultQuery("desc", "false")

	limit, err := strconv.Atoi(limitStr)
	if err != nil && limitStr != "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid limit parameter"})
		return
	}
	if limit == 0 && limitStr == "" {
		limit = 100
	}

	var since int64
	if sinceStr != "" {
		since, err = strconv.ParseInt(sinceStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid since parameter"})
			return
		}
	}

	desc, err := strconv.ParseBool(descStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid desc parameter"})
		return
	}

	posts, err := h.threadService.GetThreadPosts(c.Request.Context(), slugOrID, limit, since, sort, desc)
	if err != nil {
		if err == models.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"message": "Can't find thread with slug or id: " + slugOrID})
			return
		}

		if err.Error() == "invalid sort type" {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid sort type"})
			return
		}
		log.Printf("Error getting posts for thread %s: %v", slugOrID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, posts)
}

func (h *ThreadHandler) UpdateThreadDetails(c *gin.Context) {
	slugOrID := c.Param("slug_or_id")

	var updateData models.ThreadUpdate
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body"})
		return
	}

	updatedThread, err := h.threadService.UpdateThread(c.Request.Context(), slugOrID, updateData)
	if err != nil {
		switch err {
		case models.ErrNotFound:
			c.JSON(http.StatusNotFound, gin.H{"message": "Can't find thread with slug or id: " + slugOrID + "\n"})
			return
		default:
			log.Printf("Error updating thread %s: %v", slugOrID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
			return
		}
	}

	c.JSON(http.StatusOK, updatedThread)
}
