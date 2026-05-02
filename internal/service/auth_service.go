package service

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"stock-ai/internal/db"
	"stock-ai/internal/model"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// md5Hash 计算密码的纯 MD5 值
func md5Hash(password string) string {
	h := md5.Sum([]byte(password))
	return hex.EncodeToString(h[:])
}

// AuthService 认证服务
type AuthService struct {
	jwtSecret []byte
}

// NewAuthService 创建认证服务
func NewAuthService(secret string) *AuthService {
	return &AuthService{
		jwtSecret: []byte(secret),
	}
}

// LoginReq 登录请求
type LoginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResp 登录响应
type LoginResp struct {
	Token string       `json:"token"`
	User  model.UserInfo `json:"user"`
}

// UpdateAccountReq 更新账号请求
type UpdateAccountReq struct {
	Nickname    string `json:"nickname"`
	Avatar      string `json:"avatar"`
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// JWT Claims 自定义声明
type jwtClaims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// Login 登录（验证用户名密码，返回 JWT token）
func (svc *AuthService) Login(req *LoginReq) (*LoginResp, error) {
	user, err := db.GetUserByUsername(req.Username)
	if err != nil {
		return nil, errors.New("用户名或密码错误")
	}

	// MD5(password) 与数据库存储的 hash 比较
	inputHash := md5Hash(req.Password)
	if inputHash != user.Password {
		return nil, errors.New("用户名或密码错误")
	}

	// 检查账户状态
	if user.Status == 0 {
		return nil, errors.New("账户已禁用，请联系管理员")
	}

	// 更新最后登录时间
	_ = db.UpdateUserLastLogin(user.ID)

	// 生成 JWT token
	token, err := svc.generateToken(user.ID, user.Username)
	if err != nil {
		return nil, err
	}

	return &LoginResp{
		Token: token,
		User: model.UserInfo{
			ID:        user.ID,
			Username:  user.Username,
			Nickname:  user.Nickname,
			Avatar:    user.Avatar,
			Role:      user.Role,
			CreatedAt: user.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		},
	}, nil
}

// GetCurrentUser 根据 ID 获取用户信息
func (svc *AuthService) GetCurrentUser(userID uint) (*model.UserInfo, error) {
	user, err := db.GetUserByID(userID)
	if err != nil {
		return nil, errors.New("用户不存在")
	}

	return &model.UserInfo{
		ID:        user.ID,
		Username:  user.Username,
		Nickname:  user.Nickname,
		Avatar:    user.Avatar,
		Role:      user.Role,
		CreatedAt: user.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
	}, nil
}

// UpdateAccount 更新用户资料
func (svc *AuthService) UpdateAccount(userID uint, req *UpdateAccountReq) (*model.UserInfo, error) {
	// 如果要改密码，先验证旧密码
	if req.NewPassword != "" {
		user, err := db.GetUserByID(userID)
		if err != nil {
			return nil, errors.New("用户不存在")
		}
		oldHash := md5Hash(req.OldPassword)
		if oldHash != user.Password {
			return nil, errors.New("旧密码错误")
		}
		newHash := md5Hash(req.NewPassword)
		if err := db.UpdatePassword(userID, newHash); err != nil {
			return nil, err
		}
	}

	// 更新昵称和头像
	if err := db.UpdateUserProfile(userID, req.Nickname, req.Avatar); err != nil {
		return nil, err
	}

	return svc.GetCurrentUser(userID)
}

// generateToken 生成 JWT token
func (svc *AuthService) generateToken(userID uint, username string) (string, error) {
	claims := jwtClaims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)), // 7天有效
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "ai-stock-picker",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(svc.jwtSecret)
}

// ParseToken 解析并验证 JWT token，返回 claims 或错误
func (svc *AuthService) ParseToken(tokenString string) (*jwtClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwtClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("无效的签名方法")
		}
		return svc.jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*jwtClaims)
	if !ok || !token.Valid {
		return nil, errors.New("无效的 token")
	}

	return claims, nil
}

// Register 注册新用户（预留）
func (svc *AuthService) Register(username, password, nickname string) (*model.UserInfo, error) {
	exists, _ := db.UsernameExists(username)
	if exists {
		return nil, errors.New("用户名已存在")
	}

	hash := md5Hash(password)

	if nickname == "" {
		nickname = username
	}

	user := &model.User{
		Username:     username,
		Password:     hash,
		Nickname:     nickname,
		Status:       1,
	}

	if err := db.CreateUser(user); err != nil {
		return nil, err
	}

	return &model.UserInfo{
		ID:        user.ID,
		Username:  user.Username,
		Nickname:  user.Nickname,
		Role:      user.Role,
		CreatedAt: user.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
	}, nil
}
