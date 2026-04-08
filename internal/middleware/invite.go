package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// InviteMiddleware 邀请码中间件
type InviteMiddleware struct {
	inviteCode     string
	sessionSecret  string
	validSessions  map[string]time.Time // sessionID -> expiry
	sessionExpiry  time.Duration
}

// NewInviteMiddleware 创建邀请码中间件
func NewInviteMiddleware(inviteCode string) *InviteMiddleware {
	return &InviteMiddleware{
		inviteCode:    inviteCode,
		sessionSecret: generateSecret(),
		validSessions: make(map[string]time.Time),
		sessionExpiry: 24 * time.Hour,
	}
}

// generateSecret 生成随机密钥
func generateSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// isPublicPath 检查是否为公开路径
func (m *InviteMiddleware) isPublicPath(path string) bool {
	publicPaths := []string{
		"/api/v1/auth/invite",
		"/api/v1/health",
	}
	for _, p := range publicPaths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	// 静态资源放行
	if strings.HasPrefix(path, "/assets/") ||
		strings.HasSuffix(path, ".js") ||
		strings.HasSuffix(path, ".css") ||
		strings.HasSuffix(path, ".svg") ||
		strings.HasSuffix(path, ".png") ||
		strings.HasSuffix(path, ".ico") {
		return true
	}
	return false
}

// ValidateSession 验证会话
func (m *InviteMiddleware) isValidSession(sessionID string) bool {
	if sessionID == "" {
		return false
	}
	expiry, exists := m.validSessions[sessionID]
	if !exists {
		return false
	}
	if time.Now().After(expiry) {
		delete(m.validSessions, sessionID)
		return false
	}
	return true
}

// createSession 创建会话
func (m *InviteMiddleware) createSession() string {
	sessionID := generateSecret()
	m.validSessions[sessionID] = time.Now().Add(m.sessionExpiry)
	return sessionID
}

// Handler 返回中间件处理函数
func (m *InviteMiddleware) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 公开路径放行
		if m.isPublicPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		// 检查邀请码是否设置（未设置则放行）
		if m.inviteCode == "" {
			c.Next()
			return
		}

		// 检查会话cookie
		sessionCookie, err := c.Cookie("isdp_session")
		if err == nil && m.isValidSession(sessionCookie) {
			c.Next()
			return
		}

		// API请求返回401
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invite_required",
				"message": "请先输入邀请码",
			})
			c.Abort()
			return
		}

		// 页面请求返回邀请码输入页面
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, invitePageHTML)
		c.Abort()
	}
}

// VerifyInvite 验证邀请码
func (m *InviteMiddleware) VerifyInvite(c *gin.Context) {
	var req struct {
		Code string `json:"code"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.Code != m.inviteCode {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid invite code"})
		return
	}

	// 创建会话
	sessionID := m.createSession()

	// 设置cookie
	c.SetCookie("isdp_session", sessionID, int(m.sessionExpiry.Seconds()), "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "验证成功",
	})
}

// UpdateInviteCode 更新邀请码
func (m *InviteMiddleware) UpdateInviteCode(code string) {
	m.inviteCode = code
	// 清除所有现有会话
	m.validSessions = make(map[string]time.Time)
}

// CleanupExpiredSessions 清理过期会话
func (m *InviteMiddleware) CleanupExpiredSessions() {
	now := time.Now()
	for sessionID, expiry := range m.validSessions {
		if now.After(expiry) {
			delete(m.validSessions, sessionID)
		}
	}
}

// invitePageHTML 邀请码输入页面
const invitePageHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Colink - 请输入邀请码</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .container {
            background: white;
            padding: 40px;
            border-radius: 12px;
            box-shadow: 0 10px 40px rgba(0,0,0,0.2);
            text-align: center;
            max-width: 400px;
            width: 90%;
        }
        .logo {
            font-size: 48px;
            margin-bottom: 20px;
        }
        h1 {
            color: #333;
            margin-bottom: 10px;
            font-size: 24px;
        }
        p {
            color: #666;
            margin-bottom: 30px;
        }
        .input-group {
            margin-bottom: 20px;
        }
        input {
            width: 100%;
            padding: 12px 16px;
            border: 2px solid #e0e0e0;
            border-radius: 8px;
            font-size: 16px;
            transition: border-color 0.3s;
        }
        input:focus {
            outline: none;
            border-color: #667eea;
        }
        button {
            width: 100%;
            padding: 12px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            border: none;
            border-radius: 8px;
            font-size: 16px;
            cursor: pointer;
            transition: transform 0.2s, box-shadow 0.2s;
        }
        button:hover {
            transform: translateY(-2px);
            box-shadow: 0 4px 12px rgba(102, 126, 234, 0.4);
        }
        button:active {
            transform: translateY(0);
        }
        .error {
            color: #e74c3c;
            margin-top: 15px;
            display: none;
        }
        .error.show {
            display: block;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="logo">🔐</div>
        <h1>多智能体协作平台</h1>
        <p>请输入邀请码以访问系统</p>
        <div class="input-group">
            <input type="text" id="inviteCode" placeholder="请输入邀请码" autofocus>
        </div>
        <button onclick="submitInvite()">验证</button>
        <p class="error" id="errorMsg">邀请码错误，请重试</p>
    </div>
    <script>
        document.getElementById('inviteCode').addEventListener('keypress', function(e) {
            if (e.key === 'Enter') submitInvite();
        });
        function submitInvite() {
            const code = document.getElementById('inviteCode').value.trim();
            if (!code) return;
            fetch('/api/v1/auth/invite', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ code: code })
            })
            .then(r => r.json())
            .then(data => {
                if (data.success) {
                    window.location.reload();
                } else {
                    document.getElementById('errorMsg').classList.add('show');
                }
            })
            .catch(() => {
                document.getElementById('errorMsg').classList.add('show');
            });
        }
    </script>
</body>
</html>`