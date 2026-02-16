package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"shortlink/internal/service"
)

type Handler struct {
	Svc *service.Service
}

type shortenReq struct {
	LongURL string `json:"long_url"`
}

func (h *Handler) Register(r *gin.Engine) {
	r.POST("/shorten", h.Shorten)
	r.GET("/r/:code", h.Redirect)
	r.GET("/meta/:code", h.Meta)
}

func (h *Handler) Shorten(c *gin.Context) {
	var req shortenReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad json"})
		return
	}
	code, shortURL, err := h.Svc.Shorten(c.Request.Context(), req.LongURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": code, "short_url": shortURL})
}

func (h *Handler) Redirect(c *gin.Context) {
	code := c.Param("code")
	longURL, err := h.Svc.Resolve(c.Request.Context(), code)
	if err == service.ErrNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	c.Redirect(http.StatusFound, longURL)
}

func (h *Handler) Meta(c *gin.Context) {
	code := c.Param("code")
	pv, last, err := h.Svc.GetMeta(c.Request.Context(), code)
	if err == service.ErrNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":           code,
		"pv":             pv,
		"last_access_at": last,
	})
}

