package httpserver

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"tavily-proxy/server/internal/services"
)

func handleListAccessKeys(c *gin.Context, accessKeys *services.AccessKeyService) {
	items, err := accessKeys.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list_access_keys_failed"})
		return
	}
	c.JSON(http.StatusOK, items)
}

func handleCreateAccessKey(c *gin.Context, accessKeys *services.AccessKeyService) {
	var body struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name_required"})
		return
	}
	item, err := accessKeys.Create(c.Request.Context(), body.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create_access_key_failed"})
		return
	}
	c.JSON(http.StatusCreated, item)
}

func handleDeleteAccessKey(c *gin.Context, accessKeys *services.AccessKeyService, idStr string) {
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id"})
		return
	}
	if err := accessKeys.Delete(c.Request.Context(), uint(id)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "access_key_not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete_access_key_failed"})
		return
	}
	c.Status(http.StatusNoContent)
}
