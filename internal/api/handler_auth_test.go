package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/slurmtack/slurmtack/internal/slurm"
)

type fakeSlurmVerifier struct {
	verifyErr error
}

func (f *fakeSlurmVerifier) VerifyToken(_ context.Context, _, _ string) error {
	return f.verifyErr
}

func (f *fakeSlurmVerifier) SubmitPlaceholderJob(_ context.Context, _ slurm.PlaceholderJobRequest) (*slurm.PlaceholderJobResult, error) {
	return nil, nil
}
func (f *fakeSlurmVerifier) GetNodeState(_ context.Context, _ string) (*slurm.NodeState, error) {
	return nil, nil
}
func (f *fakeSlurmVerifier) GetNodeStateWithIdentity(_ context.Context, _ string, _ slurm.WorkloadIdentity) (*slurm.NodeState, error) {
	return nil, nil
}
func (f *fakeSlurmVerifier) DrainNode(_ context.Context, _, _ string) error { return nil }
func (f *fakeSlurmVerifier) ResumeNode(_ context.Context, _ string) error   { return nil }
func (f *fakeSlurmVerifier) CancelJob(_ context.Context, _ string) error    { return nil }
func (f *fakeSlurmVerifier) CancelJobWithIdentity(_ context.Context, _ string, _ slurm.WorkloadIdentity) error {
	return nil
}
func (f *fakeSlurmVerifier) GetJobState(_ context.Context, _ string, _ slurm.WorkloadIdentity) (*slurm.JobState, error) {
	return nil, nil
}
func (f *fakeSlurmVerifier) ListPartitions(_ context.Context) ([]slurm.Partition, error) {
	return nil, nil
}
func (f *fakeSlurmVerifier) GetNodes(_ context.Context) ([]slurm.NodeState, error) {
	return nil, nil
}

func setupAuthRouter(verifier *fakeSlurmVerifier) (*gin.Engine, *JWTManager) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	jwtMgr := NewJWTManager([]byte("test-secret-key-32-bytes-long!!"), time.Hour)
	handler := NewAuthHandler(jwtMgr, verifier)
	r.POST("/v1/auth/login", handler.Login)
	return r, jwtMgr
}

func TestLoginSuccess(t *testing.T) {
	verifier := &fakeSlurmVerifier{}
	router, jwtMgr := setupAuthRouter(verifier)

	body, _ := json.Marshal(LoginRequest{
		SlurmUser:      "alice",
		SlurmUserToken: "some-valid-token",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp LoginResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.SlurmtackToken == "" {
		t.Fatal("expected non-empty slurmtack_token")
	}

	username, err := jwtMgr.Validate(resp.SlurmtackToken)
	if err != nil {
		t.Fatalf("validate returned token: %v", err)
	}
	if username != "alice" {
		t.Fatalf("expected username alice, got %q", username)
	}
}

func TestLoginInvalidSlurmToken(t *testing.T) {
	verifier := &fakeSlurmVerifier{verifyErr: errors.New("unauthorized")}
	router, _ := setupAuthRouter(verifier)

	body, _ := json.Marshal(LoginRequest{
		SlurmUser:      "alice",
		SlurmUserToken: "expired-token",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}

	var resp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error != "invalid slurm token" {
		t.Fatalf("expected 'invalid slurm token', got %q", resp.Error)
	}
}

func TestLoginUsernameMismatch(t *testing.T) {
	verifier := &fakeSlurmVerifier{}
	router, _ := setupAuthRouter(verifier)

	// Create a fake JWT with sub=alice (base64url of {"sub":"alice"})
	// header: {"alg":"HS256","typ":"JWT"}
	// payload: {"sub":"alice"}
	fakeJWT := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJhbGljZSJ9.fakesig"

	body, _ := json.Marshal(LoginRequest{
		SlurmUser:      "bob",
		SlurmUserToken: fakeJWT,
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}

	var resp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error != "username mismatch" {
		t.Fatalf("expected 'username mismatch', got %q", resp.Error)
	}
}

func TestLoginMissingFields(t *testing.T) {
	verifier := &fakeSlurmVerifier{}
	router, _ := setupAuthRouter(verifier)

	body, _ := json.Marshal(map[string]string{"slurm_user": "alice"})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestBearerAuthRejectsStaticToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	jwtMgr := NewJWTManager([]byte("test-secret-key-32-bytes-long!!"), time.Hour)
	r := gin.New()
	r.Use(BearerAuth(jwtMgr))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{})
	})

	legacyTokens := []string{"changeme", "static-token", "my-secret-api-token"}
	for _, tok := range legacyTokens {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("legacy token %q: expected 401, got %d", tok, w.Code)
		}
	}
}

func TestBearerAuthUsernameFlowsToContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	jwtMgr := NewJWTManager([]byte("test-secret-key-32-bytes-long!!"), time.Hour)

	for _, username := range []string{"alice", "bob", "operator-1"} {
		token, _ := jwtMgr.Generate(username)

		r := gin.New()
		r.Use(BearerAuth(jwtMgr))
		r.GET("/test", func(c *gin.Context) {
			u, _ := c.Get(ContextKeyUsername)
			c.JSON(http.StatusOK, gin.H{"user": u})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("username %q: expected 200, got %d", username, w.Code)
		}

		var resp map[string]string
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["user"] != username {
			t.Fatalf("expected %q, got %q", username, resp["user"])
		}
	}
}

func TestBearerAuthJWT(t *testing.T) {
	gin.SetMode(gin.TestMode)
	jwtMgr := NewJWTManager([]byte("test-secret-key-32-bytes-long!!"), time.Hour)
	token, _ := jwtMgr.Generate("alice")

	r := gin.New()
	r.Use(BearerAuth(jwtMgr))
	r.GET("/test", func(c *gin.Context) {
		username, _ := c.Get(ContextKeyUsername)
		c.JSON(http.StatusOK, gin.H{"user": username})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["user"] != "alice" {
		t.Fatalf("expected alice, got %q", resp["user"])
	}
}

func TestBearerAuthInvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	jwtMgr := NewJWTManager([]byte("test-secret-key-32-bytes-long!!"), time.Hour)
	r := gin.New()
	r.Use(BearerAuth(jwtMgr))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestBearerAuthMissingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	jwtMgr := NewJWTManager([]byte("test-secret-key-32-bytes-long!!"), time.Hour)
	r := gin.New()
	r.Use(BearerAuth(jwtMgr))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestBearerAuthExpiredJWT(t *testing.T) {
	gin.SetMode(gin.TestMode)
	jwtMgr := NewJWTManager([]byte("test-secret-key-32-bytes-long!!"), -time.Hour)
	token, _ := jwtMgr.Generate("alice")

	r := gin.New()
	r.Use(BearerAuth(jwtMgr))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}

	var resp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error != "token expired" {
		t.Fatalf("expected 'token expired', got %q", resp.Error)
	}
}
