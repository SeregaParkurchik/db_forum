package routes

import (
	"time"

	"hardhw/internal/api"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func InitRoutes(userHandler *api.UserHandler, forumHandler *api.ForumHandler, threadHandler *api.ThreadHandler, postHandler *api.PostHandler) *gin.Engine {
	router := gin.Default()

	corsConfig := cors.Config{
		AllowOrigins:     []string{"http://localhost:8080"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}

	router.Use(cors.New(corsConfig))

	userGroup := router.Group("/user")
	{
		userGroup.POST("/:nickname/create", userHandler.CreateUser)
		userGroup.GET("/:nickname/profile", userHandler.GetUserProfile)
		userGroup.POST("/:nickname/profile", userHandler.UpdateUserProfile)
	}

	forumGroup := router.Group("/forum")
	{
		forumGroup.POST("/create", forumHandler.CreateForum)
		forumGroup.GET("/:slug/details", forumHandler.GetForumDetails)
		forumGroup.POST("/:slug/create", forumHandler.CreateThread)
		forumGroup.GET("/:slug/threads", forumHandler.GetForumThreads)
		forumGroup.GET("/:slug/users", forumHandler.GetForumUsers)
	}

	threadGroup := router.Group("/thread")
	{
		threadGroup.POST("/:slug_or_id/create", threadHandler.CreatePosts)
		threadGroup.POST("/:slug_or_id/vote", threadHandler.VoteThread)
		threadGroup.GET("/:slug_or_id/details", threadHandler.GetThreadDetails)
		threadGroup.GET("/:slug_or_id/posts", threadHandler.GetThreadPosts)
		threadGroup.POST("/:slug_or_id/details", threadHandler.UpdateThreadDetails)
	}

	postGroup := router.Group("/post")
	{
		postGroup.GET("/:id/details", postHandler.GetPostDetails)
		postGroup.POST("/:id/details", postHandler.UpdatePostDetails)
	}

	serviceGroup := router.Group("/service")
	{
		serviceGroup.POST("/clear", postHandler.ClearDatabase)
		serviceGroup.GET("/status", postHandler.GetStatus)
	}

	return router
}
