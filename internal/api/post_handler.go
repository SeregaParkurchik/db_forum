package api

import (
	"fmt"
	"hardhw/internal/models"
	"hardhw/internal/service"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type PostHandler struct {
	postService service.PostService
}

func NewPostHandler(s service.PostService) *PostHandler {
	return &PostHandler{postService: s}
}

func (h *PostHandler) GetPostDetails(c *gin.Context) {
	idStr := c.Param("id")
	postID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid post ID"})
		return
	}

	relatedStr := c.Query("related")
	var related []string
	if relatedStr != "" {
		related = strings.Split(relatedStr, ",")
	}

	if len(related) > 0 {
		response, err := h.postService.GetPostDetailsWithRelated(c.Request.Context(), postID, related)
		if err != nil {
			switch err {
			case models.ErrPostNotFound:
				c.JSON(http.StatusNotFound, gin.H{"message": fmt.Sprintf("Can't find post with id #%d\n", postID)})
				return
			case models.ErrNotFound:
				c.JSON(http.StatusNotFound, gin.H{"message": "Related user not found"})
				return
			default:
				log.Printf("Error getting post details with related for ID %d: %v", postID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
				return
			}
		}
		c.JSON(http.StatusOK, response)
		return
	}

	post, err := h.postService.GetPostDetails(c.Request.Context(), postID)
	if err != nil {
		switch err {
		case models.ErrPostNotFound:
			c.JSON(http.StatusNotFound, gin.H{"message": fmt.Sprintf("Can't find post with id #%d\n", postID)})
			return
		default:
			log.Printf("Error getting post details for ID %d: %v", postID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"post": post})
}

func (h *PostHandler) UpdatePostDetails(c *gin.Context) {
	idStr := c.Param("id")
	postID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid post ID"})
		return
	}

	var updateRequest models.PostUpdate
	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body"})
		return
	}

	updatedPost, err := h.postService.UpdatePostDetails(c.Request.Context(), postID, updateRequest.Message)
	if err != nil {
		switch err {
		case models.ErrPostNotFound:
			c.JSON(http.StatusNotFound, gin.H{"message": fmt.Sprintf("Can't find post with id #%d\n", postID)})
			return
		default:
			log.Printf("Error updating post %d: %v", postID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
			return
		}
	}

	c.JSON(http.StatusOK, updatedPost)
}

func (h *PostHandler) GetStatus(c *gin.Context) {
	status, err := h.postService.GetDatabaseStatus(c.Request.Context())
	if err != nil {
		log.Printf("Error getting database status: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
		return
	}
	c.JSON(http.StatusOK, status)
}

func (h *PostHandler) ClearDatabase(c *gin.Context) {
	err := h.postService.ClearAllData(c.Request.Context())
	if err != nil {
		log.Printf("Error clearing database: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
		return
	}
	c.Status(http.StatusOK)
}
