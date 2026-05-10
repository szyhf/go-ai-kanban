package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/xuzhiping7/ai-kanban/internal/executor"
	"github.com/xuzhiping7/ai-kanban/internal/git"
	"github.com/xuzhiping7/ai-kanban/internal/pty"
	"github.com/xuzhiping7/ai-kanban/internal/repository"
	"github.com/xuzhiping7/ai-kanban/internal/service"
)

// Handler aggregates all dependencies and registers routes.
type Handler struct {
	repoSvc    *service.RepoService
	gitSvc     *git.Service
	filesystem *service.FilesystemService
	fileSvc    *service.FileService
	eventSvc   *service.EventService
	msgStore   *service.MsgStore
	queueSvc   *service.QueuedMessageService

	repoRepo   *repository.GitRepoRepo
	tagRepo    *repository.TagRepo
	scratchRepo *repository.ScratchRepo
	sessionRepo *repository.SessionRepo
	wsRepo     *repository.WorkspaceRepo
	wsRepoRepo *repository.WorkspaceRepoRepo
	execRepo   *repository.ExecutionProcessRepo
	execStateRepo *repository.ExecutionProcessRepoStateRepo
	attachRepo *repository.AttachmentRepo
	wsAttachRepo *repository.WorkspaceAttachmentRepo

	// New services for execution and terminal.
	containerSvc *executor.ContainerService
	ptySvc       *pty.Service
	ptyHandler   *ptyHandler
}

// NewHandler creates a Handler with all dependencies injected.
func NewHandler(
	repoSvc *service.RepoService,
	gitSvc *git.Service,
	filesystem *service.FilesystemService,
	fileSvc *service.FileService,
	eventSvc *service.EventService,
	msgStore *service.MsgStore,
	queueSvc *service.QueuedMessageService,
	repoRepo *repository.GitRepoRepo,
	tagRepo *repository.TagRepo,
	scratchRepo *repository.ScratchRepo,
	sessionRepo *repository.SessionRepo,
	wsRepo *repository.WorkspaceRepo,
	wsRepoRepo *repository.WorkspaceRepoRepo,
	execRepo *repository.ExecutionProcessRepo,
	execStateRepo *repository.ExecutionProcessRepoStateRepo,
	attachRepo *repository.AttachmentRepo,
	wsAttachRepo *repository.WorkspaceAttachmentRepo,
	containerSvc *executor.ContainerService,
	ptySvc *pty.Service,
) *Handler {
	h := &Handler{
		repoSvc:    repoSvc,
		gitSvc:     gitSvc,
		filesystem: filesystem,
		fileSvc:    fileSvc,
		eventSvc:   eventSvc,
		msgStore:   msgStore,
		queueSvc:   queueSvc,
		repoRepo:   repoRepo,
		tagRepo:    tagRepo,
		scratchRepo: scratchRepo,
		sessionRepo: sessionRepo,
		wsRepo:     wsRepo,
		wsRepoRepo: wsRepoRepo,
		execRepo:   execRepo,
		execStateRepo: execStateRepo,
		attachRepo: attachRepo,
		wsAttachRepo: wsAttachRepo,
		containerSvc: containerSvc,
		ptySvc:       ptySvc,
	}
	if ptySvc != nil {
		h.ptyHandler = newPTYHandler(ptySvc)
	}
	return h
}

// RegisterRoutes registers all handler routes on the chi router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/repos", h.registerRepoRoutes)
	r.Route("/workspaces", h.registerWorkspaceRoutes)
	r.Route("/tags", h.registerTagRoutes)
	r.Route("/sessions", h.registerSessionRoutes)
	r.Route("/scratch", h.registerScratchRoutes)
	r.Route("/attachments", h.registerAttachmentRoutes)
	r.Route("/filesystem", h.registerFilesystemRoutes)
	r.Route("/execution-processes", h.registerExecutionRoutes)
	r.Get("/events", h.handleSSE)
	r.Get("/search", h.handleSearch)
	// WebSocket terminal endpoint.
	if h.ptyHandler != nil {
		r.Get("/terminal/ws", h.ptyHandler.handleTerminal)
	}
	// WebSocket approval stream.
	r.Get("/approvals/stream/ws", h.handleApprovalStreamWS)
}

// setupMultipartUpload is a helper that limits multipart form size.
func setupMultipartUpload(w http.ResponseWriter, r *http.Request, maxBytes int64) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	if err := r.ParseMultipartForm(maxBytes); err != nil {
		badRequest(w, "file too large or invalid multipart form")
		return false
	}
	return true
}
