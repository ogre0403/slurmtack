package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/slurmtack/slurmtack/internal/slurm"
)

type AuthHandler struct {
	jwtManager  *JWTManager
	slurmClient slurm.Client
}

func NewAuthHandler(jwtManager *JWTManager, slurmClient slurm.Client) *AuthHandler {
	return &AuthHandler{jwtManager: jwtManager, slurmClient: slurmClient}
}

type LoginRequest struct {
	SlurmUser      string `json:"slurm_user" binding:"required"`
	SlurmUserToken string `json:"slurm_user_token" binding:"required"`
}

type LoginResponse struct {
	SlurmtackToken string `json:"slurmtack_token"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	tokenSub, err := extractJWTSubject(req.SlurmUserToken)
	if err == nil && tokenSub != "" && tokenSub != req.SlurmUser {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "username mismatch"})
		return
	}

	if err := h.slurmClient.VerifyToken(c.Request.Context(), req.SlurmUser, req.SlurmUserToken); err != nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "invalid slurm token"})
		return
	}

	token, err := h.jwtManager.Generate(req.SlurmUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, LoginResponse{SlurmtackToken: token})
}

func extractJWTSubject(tokenString string) (string, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return "", nil
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", err
	}

	var claims struct {
		Sub      string `json:"sub"`
		Username string `json:"username"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", err
	}

	if claims.Sub != "" {
		return claims.Sub, nil
	}
	return claims.Username, nil
}
