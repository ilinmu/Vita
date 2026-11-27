package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/ilinmu/vita/internal/model"
)

// Options configures the Vita HTTP server.
type Options struct {
	JWTSecret           []byte
	AllowPublicRegister bool
	Version             string
}

// Server serves Vita's REST auth, family, and LWW incremental sync endpoints.
type Server struct {
	db   *DB
	opts Options
	mux  *http.ServeMux
}

// New constructs a Server with registered routes.
func New(db *DB, opts Options) *Server {
	if opts.Version == "" {
		opts.Version = "0.2.0"
	}
	s := &Server{
		db:   db,
		opts: opts,
		mux:  http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("POST /api/v1/auth/register", s.handleRegister)
	s.mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	s.mux.HandleFunc("GET /api/v1/me", s.handleMe)
	s.mux.HandleFunc("POST /api/v1/families", s.handleCreateFamily)
	s.mux.HandleFunc("POST /api/v1/families/join", s.handleJoinFamily)
	s.mux.HandleFunc("POST /api/v1/sync", s.handleSync)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": s.opts.Version,
	})
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req model.AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效的请求格式")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if len(req.Username) < 2 || len(req.Password) < 6 {
		writeError(w, http.StatusBadRequest, "用户名至少 2 个字符，密码至少 6 位")
		return
	}

	// Gate public registration when AllowPublicRegister is false:
	// allow first owner (0 users) or someone holding a valid family invite code.
	if !s.opts.AllowPublicRegister {
		count, err := s.db.UserCount(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if count > 0 {
			ok, err := s.db.InviteCodeExists(r.Context(), req.InviteCode)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !ok {
				writeError(w, http.StatusForbidden, "服务端已关闭公开注册，请向家人获取家庭邀请码")
				return
			}
		}
	}

	hash, err := hashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	u, err := s.db.CreateUser(r.Context(), req.Username, hash)
	if errors.Is(err, ErrConflict) {
		writeError(w, http.StatusConflict, "用户名已存在")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var families []model.FamilyInfo
	if code := strings.TrimSpace(req.InviteCode); code != "" {
		f, err := s.db.JoinFamilyByCode(r.Context(), u.ID, code)
		if err == nil {
			families = append(families, f)
		}
	}
	if len(families) == 0 {
		fname := strings.TrimSpace(req.FamilyName)
		if fname == "" {
			fname = u.Username + "的家庭"
		}
		f, err := s.db.CreateFamily(r.Context(), u.ID, fname)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		families = append(families, f)
	}

	tok, err := signJWT(s.opts.JWTSecret, u.ID, u.Username, 90*24*time.Hour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, model.AuthResponse{
		Token:    tok,
		User:     u,
		Families: families,
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req model.AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效的请求格式")
		return
	}
	u, hash, err := s.db.FindUserByUsername(r.Context(), req.Username)
	if errors.Is(err, ErrNotFound) || !checkPassword(hash, req.Password) {
		writeError(w, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	families, err := s.db.UserFamilies(r.Context(), u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	tok, err := signJWT(s.opts.JWTSecret, u.ID, u.Username, 90*24*time.Hour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.AuthResponse{
		Token:    tok,
		User:     u,
		Families: families,
	})
}

func (s *Server) authenticate(r *http.Request) (jwtClaims, error) {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return jwtClaims{}, ErrUnauthorized
	}
	tok := strings.TrimSpace(auth[7:])
	claims, err := verifyJWT(s.opts.JWTSecret, tok)
	if err != nil {
		return jwtClaims{}, ErrUnauthorized
	}
	return claims, nil
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authenticate(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	u, err := s.db.FindUserByID(r.Context(), claims.Sub)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "用户不存在")
		return
	}
	families, err := s.db.UserFamilies(r.Context(), u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.AuthResponse{
		User:     u,
		Families: families,
	})
}

func (s *Server) handleCreateFamily(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authenticate(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	var req model.CreateFamilyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效的请求格式")
		return
	}
	f, err := s.db.CreateFamily(r.Context(), claims.Sub, req.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, f)
}

func (s *Server) handleJoinFamily(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authenticate(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	var req model.JoinFamilyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效的请求格式")
		return
	}
	f, err := s.db.JoinFamilyByCode(r.Context(), claims.Sub, req.InviteCode)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "邀请码无效")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, f)
}

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	claims, err := s.authenticate(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "未登录或登录已过期")
		return
	}
	familyID := strings.TrimSpace(r.Header.Get("X-Family-ID"))
	if familyID == "" {
		fams, err := s.db.UserFamilies(r.Context(), claims.Sub)
		if err != nil || len(fams) == 0 {
			writeError(w, http.StatusBadRequest, "缺少家庭 ID")
			return
		}
		familyID = fams[0].ID
	}
	var req model.SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效的同步请求")
		return
	}
	resp, err := s.db.Sync(r.Context(), familyID, claims.Sub, req)
	if errors.Is(err, ErrForbidden) {
		writeError(w, http.StatusForbidden, "无权访问该家庭空间")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
