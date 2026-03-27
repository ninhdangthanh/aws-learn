package controller

import (
	"net/http"
	"strconv"

	"github.com/go-template/service"

	"github.com/gin-gonic/gin"
)

type ProductController struct {
	svc service.ProductService
}

func NewProductController() *ProductController {
	return &ProductController{svc: service.NewProductService()}
}

func (ctrl *ProductController) CreateProduct(c *gin.Context) {
	var product struct {
		SKU   string  `json:"sku" binding:"required"`
		Name  string  `json:"name" binding:"required"`
		Price float64 `json:"price" binding:"required"`
		Stock int     `json:"stock"`
	}
	if err := c.ShouldBindJSON(&product); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Note: Mapping models.Product elsewhere, simplifies here.
	c.JSON(http.StatusCreated, product)
}

func (ctrl *ProductController) GetProduct(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	product, err := ctrl.svc.GetProduct(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}
	c.JSON(http.StatusOK, product)
}
