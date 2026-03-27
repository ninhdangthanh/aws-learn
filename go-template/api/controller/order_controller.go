package controller

import (
	"net/http"
	"strconv"

	"github.com/go-template/models"
	"github.com/go-template/service"

	"github.com/gin-gonic/gin"
)

type OrderController struct {
	svc service.OrderService
}

func NewOrderController() *OrderController {
	return &OrderController{svc: service.NewOrderService()}
}

func (ctrl *OrderController) CreateOrder(c *gin.Context) {
	var order models.Order
	if err := c.ShouldBindJSON(&order); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := ctrl.svc.CreateOrder(c.Request.Context(), &order); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create order"})
		return
	}
	c.JSON(http.StatusCreated, order)
}

func (ctrl *OrderController) GetOrder(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	order, err := ctrl.svc.GetOrder(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}
	c.JSON(http.StatusOK, order)
}
