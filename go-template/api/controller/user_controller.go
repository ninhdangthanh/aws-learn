package controller

import (
	"net/http"
	"strconv"

	"github.com/go-template/models"
	"github.com/go-template/service"

	"github.com/gin-gonic/gin"
)

type UserController struct {
	svc service.UserService
}

func NewUserController() *UserController {
	return &UserController{svc: service.NewUserService()}
}

func (ctrl *UserController) CreateUser(c *gin.Context) {
	var user models.User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := ctrl.svc.CreateUser(c.Request.Context(), &user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}
	c.JSON(http.StatusCreated, user)
}

func (ctrl *UserController) GetUser(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	user, err := ctrl.svc.GetUser(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}
