package api

import (
	"errors"
	"fmt"
	"hardhw/internal/models"
	"hardhw/internal/service"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userService service.UserService
}

func NewUserHandler(s service.UserService) *UserHandler {
	return &UserHandler{userService: s}
}

func (h *UserHandler) CreateUser(c *gin.Context) {
	nickname := c.Param("nickname")

	var newUser models.User

	if err := c.ShouldBindJSON(&newUser); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Неверное тело запроса", "error": err.Error()})
		return
	}

	newUser.Nickname = nickname

	createdUser, conflictUsers, err := h.userService.CreateUser(c.Request.Context(), newUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Ошибка при создании user", "error": err.Error()})
		return
	}

	if len(conflictUsers) > 0 {
		c.JSON(409, conflictUsers)
		return
	}

	c.JSON(http.StatusCreated, createdUser)
}

func (h *UserHandler) GetUserProfile(c *gin.Context) {
	nickname := c.Param("nickname")

	user, err := h.userService.GetUserByNickname(c.Request.Context(), nickname)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": fmt.Sprintf("Can't find user with nickname: %s", nickname)})
			return
		}
		log.Printf("Error getting user profile: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *UserHandler) UpdateUserProfile(c *gin.Context) {
	nickname := c.Param("nickname")

	var updatedUserData models.User
	if err := c.ShouldBindJSON(&updatedUserData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Неверное тело запроса", "error": err.Error()})
		return
	}

	user, err := h.userService.UpdateUser(c.Request.Context(), nickname, updatedUserData)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": fmt.Sprintf("Can't find user with nickname: %s", nickname)})
			return
		}
		if errors.Is(err, models.ErrUserConflict) {
			c.JSON(http.StatusConflict, gin.H{"message": fmt.Sprintf("Email %s already in use.", updatedUserData.Email)})
			return
		}
		log.Printf("Ошибка при обновлении профиля пользователя: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Ошибка при обновлении профиля пользователя", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}
