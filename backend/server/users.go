package server

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"zui/storage"
)

type UsersHandler struct {
	store *storage.Store
}

func NewUsersHandler(store *storage.Store) *UsersHandler {
	return &UsersHandler{store: store}
}

type userResponse struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

var userRoles = []string{"owner", "admin", "operator", "viewer"}

func toUserResponse(u storage.User) userResponse {
	return userResponse{
		ID:        u.ID,
		Username:  u.Username,
		Role:      strings.ToLower(strings.TrimSpace(u.Role)),
		Status:    strings.ToLower(strings.TrimSpace(u.Status)),
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

func normalizeUserRole(role string) string {
	role = strings.ToLower(strings.TrimSpace(role))
	for _, allowed := range userRoles {
		if role == allowed {
			return role
		}
	}
	return "viewer"
}

func hasAdminPrivilege(role string) bool {
	role = strings.ToLower(strings.TrimSpace(role))
	return role == "owner" || role == "admin"
}

func isOwnerRole(role string) bool {
	return strings.EqualFold(strings.TrimSpace(role), "owner")
}

func getContextRole(c *gin.Context) string {
	if v, ok := c.Get("auth.role"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getContextUserID(c *gin.Context) int64 {
	if v, ok := c.Get("auth.userID"); ok {
		if id, ok := toInt64(v); ok {
			return id
		}
	}
	return 0
}

func (h *UsersHandler) ensureAdmin(c *gin.Context) (string, bool) {
	role := getContextRole(c)
	if !hasAdminPrivilege(role) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient role"})
		return role, false
	}
	return role, true
}

func parseUserIDParam(c *gin.Context) (int64, bool) {
	idStr := strings.TrimSpace(c.Param("id"))
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return 0, false
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return 0, false
	}
	return id, true
}

func (h *UsersHandler) List(c *gin.Context) {
	if _, ok := h.ensureAdmin(c); !ok {
		return
	}
	items, err := h.store.ListUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp := make([]userResponse, 0, len(items))
	for _, item := range items {
		resp = append(resp, toUserResponse(item))
	}
	c.JSON(http.StatusOK, gin.H{"items": resp})
}

func (h *UsersHandler) Create(c *gin.Context) {
	actorRole, ok := h.ensureAdmin(c)
	if !ok {
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json body"})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || strings.TrimSpace(req.Password) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password are required"})
		return
	}
	if err := validatePasswordStrength(req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	role := normalizeUserRole(req.Role)
	if role == "owner" && !isOwnerRole(actorRole) {
		c.JSON(http.StatusForbidden, gin.H{"error": "only owner can create owner role"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	user, err := h.store.CreateUser(req.Username, string(hash), role)
	if err != nil {
		if isUniqueConstraintError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "username already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	recordAudit(c, h.store, "user.create", strconv.FormatInt(user.ID, 10), user.Username+"/"+role)
	c.JSON(http.StatusCreated, gin.H{"user": toUserResponse(user)})
}

func (h *UsersHandler) Update(c *gin.Context) {
	actorRole, ok := h.ensureAdmin(c)
	if !ok {
		return
	}
	userID, ok := parseUserIDParam(c)
	if !ok {
		return
	}
	var req struct {
		Role string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json body"})
		return
	}
	role := normalizeUserRole(req.Role)
	if role == "owner" && !isOwnerRole(actorRole) {
		c.JSON(http.StatusForbidden, gin.H{"error": "only owner can assign owner role"})
		return
	}
	user, err := h.store.GetUserByID(userID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if isOwnerRole(user.Role) && role != "owner" {
		owners, err := h.store.CountUsersByRole("owner")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if owners <= 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cannot remove the last owner"})
			return
		}
	}
	updated, err := h.store.UpdateUserRole(userID, role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	recordAudit(c, h.store, "user.update_role", strconv.FormatInt(userID, 10), role)
	c.JSON(http.StatusOK, gin.H{"user": toUserResponse(updated)})
}

func (h *UsersHandler) ResetPassword(c *gin.Context) {
	actorRole, ok := h.ensureAdmin(c)
	if !ok {
		return
	}
	userID, ok := parseUserIDParam(c)
	if !ok {
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json body"})
		return
	}
	if err := validatePasswordStrength(req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := h.store.GetUserByID(userID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if isOwnerRole(user.Role) && !isOwnerRole(actorRole) {
		c.JSON(http.StatusForbidden, gin.H{"error": "only owner can reset owner password"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := h.store.UpdateUserPassword(userID, string(hash)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	user.PasswordHash = string(hash)
	if isOwnerRole(user.Role) {
		h.store.SyncLegacyAdminUser(user)
	}
	recordAudit(c, h.store, "user.reset_password", strconv.FormatInt(userID, 10), "")
	c.JSON(http.StatusOK, gin.H{"message": "password reset"})
}

func (h *UsersHandler) Delete(c *gin.Context) {
	actorRole, ok := h.ensureAdmin(c)
	if !ok {
		return
	}
	userID, ok := parseUserIDParam(c)
	if !ok {
		return
	}
	if userID == getContextUserID(c) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete current user"})
		return
	}
	user, err := h.store.GetUserByID(userID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if isOwnerRole(user.Role) {
		if !isOwnerRole(actorRole) {
			c.JSON(http.StatusForbidden, gin.H{"error": "only owner can delete owner account"})
			return
		}
		owners, err := h.store.CountUsersByRole("owner")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if owners <= 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete the last owner"})
			return
		}
	}
	if err := h.store.DeleteUser(userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	recordAudit(c, h.store, "user.delete", strconv.FormatInt(userID, 10), user.Username)
	c.JSON(http.StatusOK, gin.H{"message": "user deleted"})
}
