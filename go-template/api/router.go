package api

import (
	"go-template/api/controller"
	"go-template/api/middleware"
	"go-template/config"

	"github.com/gin-gonic/gin"
)

func SetupRouter(cfg *config.Config) *gin.Engine {
	r := gin.Default()

	// Initialize Controllers
	orderCtrl := controller.NewOrderController()
	userCtrl := controller.NewUserController()
	productCtrl := controller.NewProductController()
	authCtrl := controller.NewAuthController(cfg)

	// Public Routes
	r.POST("/users", userCtrl.CreateUser)
	r.POST("/login", authCtrl.Login)
	r.POST("/refresh", authCtrl.Refresh)

	// Protected Routes
	auth := r.Group("/")
	auth.Use(middleware.AuthMiddleware(cfg.JWTSecret))
	{
		// Auth Actions
		auth.POST("/logout", authCtrl.Logout)
		auth.POST("/evict/:id", authCtrl.EvictUser)

		// Orders
		auth.POST("/orders", orderCtrl.CreateOrder)
		auth.GET("/orders/:id", orderCtrl.GetOrder)

		// Users
		auth.GET("/users/:id", userCtrl.GetUser)

		// Products
		auth.POST("/products", productCtrl.CreateProduct)
		auth.GET("/products/:id", productCtrl.GetProduct)
	}

	return r
}
