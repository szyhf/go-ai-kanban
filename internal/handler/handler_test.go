package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/xuzhiping7/ai-kanban/internal/database"
	"github.com/xuzhiping7/ai-kanban/internal/git"
	"github.com/xuzhiping7/ai-kanban/internal/repository"
	"github.com/xuzhiping7/ai-kanban/internal/service"
)

// setupTestHandler creates a Handler with a test database and all dependencies.
func setupTestHandler(t *testing.T) (*Handler, *chi.Mux) {
	t.Helper()

	db, err := database.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	gitSvc := git.NewService()
	msgStore := service.NewMsgStore()
	eventSvc := service.NewEventService(msgStore, db.DB)

	repoRepo := repository.NewGitRepoRepo(db.DB)
	tagRepo := repository.NewTagRepo(db.DB)
	scratchRepo := repository.NewScratchRepo(db.DB)
	sessionRepo := repository.NewSessionRepo(db.DB)
	wsRepo := repository.NewWorkspaceRepo(db.DB)
	wsRepoRepo := repository.NewWorkspaceRepoRepo(db.DB)
	execRepo := repository.NewExecutionProcessRepo(db.DB)
	execStateRepo := repository.NewExecutionProcessRepoStateRepo(db.DB)
	attachRepo := repository.NewAttachmentRepo(db.DB)
	wsAttachRepo := repository.NewWorkspaceAttachmentRepo(db.DB)

	cacheDir := t.TempDir()
	fileSvc := service.NewFileService(cacheDir, attachRepo, wsAttachRepo)
	repoSvc := service.NewRepoService(repoRepo, gitSvc)
	filesystemSvc := service.NewFilesystemService(gitSvc)
	queueSvc := service.NewQueuedMessageService()

	h := NewHandler(
		repoSvc, gitSvc, filesystemSvc, fileSvc, eventSvc, msgStore, queueSvc,
		repoRepo, tagRepo, scratchRepo, sessionRepo, wsRepo, wsRepoRepo,
		execRepo, execStateRepo, attachRepo, wsAttachRepo,
		nil, nil, nil, // containerSvc, ptySvc, turnRepo — not needed in tests
	)

	r := chi.NewRouter()
	r.Route("/api", h.RegisterRoutes)

	return h, r
}

// decodeResponse decodes the standard API response envelope.
func decodeResponse(t *testing.T, body io.Reader) (ApiResponse, map[string]interface{}) {
	t.Helper()
	var raw struct {
		Success bool                   `json:"success"`
		Data    json.RawMessage        `json:"data"`
		Message string                 `json:"message"`
		Error   string                 `json:"error_data,omitempty"`
	}
	if err := json.NewDecoder(body).Decode(&raw); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	resp := ApiResponse{Success: raw.Success, Message: raw.Message}
	var data map[string]interface{}
	if raw.Data != nil {
		json.Unmarshal(raw.Data, &data)
	}
	return resp, data
}

// decodeResponseSlice decodes the data field as a slice.
func decodeResponseSlice(t *testing.T, body io.Reader) (bool, []interface{}) {
	t.Helper()
	var raw struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(body).Decode(&raw); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	var data []interface{}
	if raw.Data != nil {
		json.Unmarshal(raw.Data, &data)
	}
	return raw.Success, data
}

// doRequest performs a test HTTP request.
func doRequest(t *testing.T, router *chi.Mux, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		req = httptest.NewRequest(method, path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// ApiResponse mirrors the httputil.ApiResponse for test assertions.
type ApiResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

func TestHandler_RegisterRoutes(t *testing.T) {
	_, router := setupTestHandler(t)

	// Verify routes are registered by making requests.
	tests := []struct {
		method, path string
		wantCode     int
	}{
		{http.MethodGet, "/api/repos", http.StatusOK},
		{http.MethodGet, "/api/tags", http.StatusOK},
		{http.MethodGet, "/api/sessions", http.StatusBadRequest}, // requires workspace_id
		{http.MethodGet, "/api/scratch", http.StatusOK},
		{http.MethodGet, "/api/filesystem/directory", http.StatusOK}, // falls back to home
		{http.MethodGet, "/api/search", http.StatusBadRequest},                       // requires q
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != tt.wantCode {
				t.Errorf("expected %d, got %d; body: %s", tt.wantCode, rec.Code, rec.Body.String())
			}
		})
	}
}

// --- Repo Handler Tests ---

func TestRepoHandler_ListRepos(t *testing.T) {
	_, router := setupTestHandler(t)
	rec := doRequest(t, router, http.MethodGet, "/api/repos", nil)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	ok, data := decodeResponseSlice(t, rec.Body)
	if !ok {
		t.Error("expected success=true")
	}
	if data == nil {
		data = []interface{}{}
	}
	if len(data) != 0 {
		t.Errorf("expected empty list, got %d items", len(data))
	}
}

func TestRepoHandler_RegisterAndGet(t *testing.T) {
	_, router := setupTestHandler(t)
	gitSvc := git.NewService()

	// Create a real git repo.
	repoDir := t.TempDir()
	if err := gitSvc.InitializeRepoWithMainBranch(repoDir); err != nil {
		t.Fatalf("init repo: %v", err)
	}

	// Register repo.
	body := map[string]string{"path": repoDir, "display_name": "test-repo"}
	rec := doRequest(t, router, http.MethodPost, "/api/repos", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var createResp struct {
		Success bool                   `json:"success"`
		Data    map[string]interface{} `json:"data"`
	}
	json.NewDecoder(rec.Body).Decode(&createResp)
	if !createResp.Success {
		t.Error("expected success")
	}

	id, _ := createResp.Data["id"].(string)
	if id == "" {
		t.Fatal("expected repo id")
	}

	// Get repo by ID.
	rec = doRequest(t, router, http.MethodGet, "/api/repos/"+id, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestRepoHandler_GetNotFound(t *testing.T) {
	_, router := setupTestHandler(t)

	rec := doRequest(t, router, http.MethodGet, "/api/repos/00000000-0000-0000-0000-000000000000", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestRepoHandler_DeleteRepo(t *testing.T) {
	_, router := setupTestHandler(t)
	gitSvc := git.NewService()

	repoDir := t.TempDir()
	if err := gitSvc.InitializeRepoWithMainBranch(repoDir); err != nil {
		t.Fatalf("init repo: %v", err)
	}
	body := map[string]string{"path": repoDir, "display_name": "delete-me"}
	rec := doRequest(t, router, http.MethodPost, "/api/repos", body)
	var createResp struct {
		Data map[string]interface{} `json:"data"`
	}
	json.NewDecoder(rec.Body).Decode(&createResp)
	id, _ := createResp.Data["id"].(string)

	// Delete.
	rec = doRequest(t, router, http.MethodDelete, "/api/repos/"+id, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	// Verify deleted.
	rec = doRequest(t, router, http.MethodGet, "/api/repos/"+id, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", rec.Code)
	}
}

// --- Tag Handler Tests ---

func TestTagHandler_CRUD(t *testing.T) {
	_, router := setupTestHandler(t)

	// Create tag.
	body := map[string]string{"tag_name": "bug", "content": "bug fix"}
	rec := doRequest(t, router, http.MethodPost, "/api/tags", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var createResp struct {
		Data map[string]interface{} `json:"data"`
	}
	json.NewDecoder(rec.Body).Decode(&createResp)
	id, _ := createResp.Data["id"].(string)

	// List tags.
	rec = doRequest(t, router, http.MethodGet, "/api/tags", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	// Get tag.
	rec = doRequest(t, router, http.MethodGet, "/api/tags/"+id, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	// Update tag.
	body = map[string]string{"tag_name": "feature", "content": "new feature"}
	rec = doRequest(t, router, http.MethodPut, "/api/tags/"+id, body)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	// Delete tag.
	rec = doRequest(t, router, http.MethodDelete, "/api/tags/"+id, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

// --- Scratch Handler Tests ---

func TestScratchHandler_ListAndUpsert(t *testing.T) {
	_, router := setupTestHandler(t)

	// List all (empty).
	rec := doRequest(t, router, http.MethodGet, "/api/scratch", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	// Generate a scratch ID.
	scratchID := "00000000-0000-0000-0000-000000000001"
	body := map[string]interface{}{
		"type": "WORKSPACE_NOTES",
		"data": map[string]string{"content": "hello"},
	}
	rec = doRequest(t, router, http.MethodPut, "/api/scratch/WORKSPACE_NOTES/"+scratchID, body)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

// --- Workspace Handler Tests ---

func TestWorkspaceHandler_ListAndCreate(t *testing.T) {
	_, router := setupTestHandler(t)

	// List (empty).
	rec := doRequest(t, router, http.MethodGet, "/api/workspaces", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	// Create workspace.
	body := map[string]interface{}{
		"branch": "feature-test",
		"name":   "test-workspace",
	}
	rec = doRequest(t, router, http.MethodPost, "/api/workspaces", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

// --- Attachment Handler Tests ---

func TestAttachmentHandler_UploadAndDownload(t *testing.T) {
	_, router := setupTestHandler(t)

	// Create a multipart form with a file.
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", "test.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	part.Write([]byte("hello world"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/attachments/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var uploadResp struct {
		Data map[string]interface{} `json:"data"`
	}
	json.NewDecoder(rec.Body).Decode(&uploadResp)
	id, _ := uploadResp.Data["id"].(string)

	// Download file.
	req = httptest.NewRequest(http.MethodGet, "/api/attachments/"+id+"/file", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

// --- Filesystem Handler Tests ---

func TestFilesystemHandler_ListDirectory(t *testing.T) {
	_, router := setupTestHandler(t)

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hi"), 0o644)

	req := httptest.NewRequest(http.MethodGet, "/api/filesystem/directory?path="+dir, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

// --- Event Handler Tests ---

func TestEventHandler_SSE(t *testing.T) {
	_, router := setupTestHandler(t)

	// Use a cancellable context so SSE doesn't block forever.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	// Run SSE in a goroutine and cancel context after a short delay.
	done := make(chan struct{})
	go func() {
		router.ServeHTTP(rec, req)
		close(done)
	}()

	// Give SSE a moment to write headers, then cancel.
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	ct := rec.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Errorf("expected text/event-stream, got %q", ct)
	}
}

// --- Session Handler Tests ---

func TestSessionHandler_CreateAndGet(t *testing.T) {
	_, router := setupTestHandler(t)

	// Create a workspace first.
	wsBody := map[string]interface{}{
		"branch": "session-test",
		"name":   "ws-for-session",
	}
	rec := doRequest(t, router, http.MethodPost, "/api/workspaces", wsBody)
	var wsResp struct {
		Data map[string]interface{} `json:"data"`
	}
	json.NewDecoder(rec.Body).Decode(&wsResp)
	wsID, _ := wsResp.Data["id"].(string)

	// Create session.
	body := map[string]interface{}{
		"workspace_id": wsID,
		"name":         "test-session",
	}
	rec = doRequest(t, router, http.MethodPost, "/api/sessions", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var sessResp struct {
		Data map[string]interface{} `json:"data"`
	}
	json.NewDecoder(rec.Body).Decode(&sessResp)
	sessID, _ := sessResp.Data["id"].(string)

	// List sessions.
	rec = doRequest(t, router, http.MethodGet, "/api/sessions?workspace_id="+wsID, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	// Get session.
	rec = doRequest(t, router, http.MethodGet, "/api/sessions/"+sessID, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

// --- Execution Handler Tests ---

func TestExecutionHandler_GetNotFound(t *testing.T) {
	_, router := setupTestHandler(t)

	rec := doRequest(t, router, http.MethodGet, "/api/execution-processes/00000000-0000-0000-0000-000000000000", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

// --- Search Handler Tests ---

func TestSearchHandler_MissingQuery(t *testing.T) {
	_, router := setupTestHandler(t)

	rec := doRequest(t, router, http.MethodGet, "/api/search", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

// --- Helpers Tests ---

func TestDecodeJSON_Invalid(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")

	result := decodeJSON(w, req, &struct{}{})
	if result {
		t.Error("expected false for invalid JSON")
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestParseUUID_Invalid(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/{id}", func(w http.ResponseWriter, r *http.Request) {
		_, ok := parseUUID(w, r, "id")
		if ok {
			t.Error("expected false for invalid UUID")
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/not-a-uuid", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

// Make sure slog.Default() is used in tests to avoid nil logger.
func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
}
