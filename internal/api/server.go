package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"elk-local/internal/config"
	"elk-local/internal/environment"
	"elk-local/internal/integration"
)

type folderOpener interface {
	OpenFolder(path string) error
}

type Server struct {
	projectRoot string
	lifecycle   environment.LifecycleService
	backup      environment.BackupService
	staticDir   string
	installer   environment.ApplicationInstaller
	editor      folderOpener
	explorer    folderOpener
	httpServer  *http.Server
	serverMu    sync.RWMutex
}

type EnvironmentView struct {
	Name         string          `json:"name"`
	Preset       string          `json:"preset"`
	Application  ApplicationView `json:"application"`
	ProjectType  string          `json:"projectType"`
	ProjectRoot  string          `json:"projectRoot"`
	ComposePath  string          `json:"composePath"`
	StoragePath  string          `json:"storagePath"`
	PHPVersion   string          `json:"phpVersion"`
	WebServer    string          `json:"webServer"`
	Database     DatabaseView    `json:"database"`
	Tooling      ToolingView     `json:"tooling"`
	Network      NetworkView     `json:"network"`
	URLs         URLView         `json:"urls"`
	Status       StatusView      `json:"status"`
	UpdatedAt    string          `json:"updatedAt"`
	ManifestPath string          `json:"manifestPath"`
}

type ApplicationView struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

type DatabaseView struct {
	Engine   string `json:"engine"`
	Version  string `json:"version"`
	Name     string `json:"name"`
	User     string `json:"user"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	RootUser string `json:"rootUser"`
}

type ToolingView struct {
	Adminer ToolToggleView `json:"adminer"`
	Mailpit ToolToggleView `json:"mailpit"`
	Xdebug  XdebugView     `json:"xdebug"`
}

type ToolToggleView struct {
	Enabled  bool `json:"enabled"`
	Port     int  `json:"port,omitempty"`
	SMTPPort int  `json:"smtpPort,omitempty"`
}

type XdebugView struct {
	Enabled    bool   `json:"enabled"`
	ClientHost string `json:"clientHost,omitempty"`
	ClientPort int    `json:"clientPort,omitempty"`
}

type NetworkView struct {
	HTTPPort     int `json:"httpPort"`
	DatabasePort int `json:"databasePort"`
}

type URLView struct {
	App      string `json:"app,omitempty"`
	Adminer  string `json:"adminer,omitempty"`
	Mailpit  string `json:"mailpit,omitempty"`
	Database string `json:"database,omitempty"`
	SMTP     string `json:"smtp,omitempty"`
}

type StatusView struct {
	State      string          `json:"state"`
	Containers []ContainerView `json:"containers"`
	Error      string          `json:"error,omitempty"`
	RawOutput  string          `json:"rawOutput,omitempty"`
}

type ContainerView struct {
	Name           string   `json:"name"`
	Service        string   `json:"service"`
	State          string   `json:"state"`
	Health         string   `json:"health,omitempty"`
	PublishedPorts []string `json:"publishedPorts,omitempty"`
}

type composeContainer struct {
	Name       string             `json:"Name"`
	Service    string             `json:"Service"`
	State      string             `json:"State"`
	Health     string             `json:"Health"`
	Publishers []composePublisher `json:"Publishers"`
}

type composePublisher struct {
	URL           string `json:"URL"`
	TargetPort    int    `json:"TargetPort"`
	PublishedPort int    `json:"PublishedPort"`
	Protocol      string `json:"Protocol"`
}

type environmentsResponse struct {
	ProjectRoot            string            `json:"projectRoot"`
	DefaultProjectRootBase string            `json:"defaultProjectRootBase"`
	Environments           []EnvironmentView `json:"environments"`
	PresetOptions          []PresetView      `json:"presetOptions,omitempty"`
}

type environmentResponse struct {
	Environment EnvironmentView `json:"environment"`
	Output      string          `json:"output,omitempty"`
}

type deleteEnvironmentResponse struct {
	Name                string `json:"name"`
	RemovedProjectFiles bool   `json:"removedProjectFiles,omitempty"`
	RemovedBackups      bool   `json:"removedBackups,omitempty"`
}

type backupInventoryResponse struct {
	EnvironmentName string       `json:"environmentName"`
	Backups         []BackupView `json:"backups"`
}

type backupActionResponse struct {
	EnvironmentName      string       `json:"environmentName"`
	Backup               BackupView   `json:"backup"`
	Backups              []BackupView `json:"backups,omitempty"`
	RestoredProjectFiles bool         `json:"restoredProjectFiles,omitempty"`
	Message              string       `json:"message,omitempty"`
}

type BackupView struct {
	FileName             string `json:"fileName"`
	Path                 string `json:"path"`
	SizeBytes            int64  `json:"sizeBytes"`
	CreatedAt            string `json:"createdAt"`
	EnvironmentName      string `json:"environmentName,omitempty"`
	DatabaseName         string `json:"databaseName,omitempty"`
	IncludesProjectFiles bool   `json:"includesProjectFiles"`
	Error                string `json:"error,omitempty"`
}

type PresetView struct {
	Name              string `json:"name"`
	Description       string `json:"description"`
	ApplicationName   string `json:"applicationName,omitempty"`
	DefaultAppVersion string `json:"defaultAppVersion,omitempty"`
	AppVersionHint    string `json:"appVersionHint,omitempty"`
	ProjectType       string `json:"projectType"`
	PHPVersion        string `json:"phpVersion"`
	WebServer         string `json:"webServer"`
	DatabaseEngine    string `json:"databaseEngine"`
	DatabaseVersion   string `json:"databaseVersion"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type createEnvironmentRequest struct {
	Name                 string `json:"name"`
	Preset               string `json:"preset"`
	ApplicationVersion   string `json:"applicationVersion"`
	ProjectRoot          string `json:"projectRoot"`
	PHPVersion           string `json:"phpVersion"`
	WebServer            string `json:"webServer"`
	DatabaseEngine       string `json:"databaseEngine"`
	DatabaseVersion      string `json:"databaseVersion"`
	DatabaseName         string `json:"databaseName"`
	DatabaseUser         string `json:"databaseUser"`
	DatabasePassword     string `json:"databasePassword"`
	DatabaseRootPassword string `json:"databaseRootPassword"`
	AdminerEnabled       bool   `json:"adminerEnabled"`
	MailpitEnabled       bool   `json:"mailpitEnabled"`
	XdebugEnabled        bool   `json:"xdebugEnabled"`
	Force                bool   `json:"force"`
}

type createBackupRequest struct {
	OutputPath          string `json:"outputPath"`
	IncludeProjectFiles bool   `json:"includeProjectFiles"`
	Force               bool   `json:"force"`
}

type importBackupRequest struct {
	ArchivePath string `json:"archivePath"`
	Force       bool   `json:"force"`
}

type restoreBackupRequest struct {
	ArchivePath         string `json:"archivePath"`
	RestoreProjectFiles bool   `json:"restoreProjectFiles"`
	Force               bool   `json:"force"`
}

func NewServer(projectRoot string, staticDir string, executor environment.ComposeInputExecutor, installer environment.ApplicationInstaller) (*Server, error) {
	resolvedProjectRoot, err := resolveProjectRoot(projectRoot)
	if err != nil {
		return nil, err
	}

	resolvedStaticDir := ""
	if strings.TrimSpace(staticDir) != "" {
		absStaticDir, err := filepath.Abs(staticDir)
		if err != nil {
			return nil, fmt.Errorf("resolve static dir: %w", err)
		}

		if info, err := os.Stat(absStaticDir); err == nil && info.IsDir() {
			resolvedStaticDir = absStaticDir
		}
	}

	return &Server{
		projectRoot: resolvedProjectRoot,
		lifecycle:   environment.NewLifecycleService(executor),
		backup:      environment.NewBackupService(executor),
		staticDir:   resolvedStaticDir,
		installer:   installer,
		editor:      integration.VSCodeOpener{},
		explorer:    integration.SystemFolderOpener{},
	}, nil
}

func (server *Server) Handler() http.Handler {
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/daemon/status", server.handleDaemonStatus)
	apiMux.HandleFunc("/api/daemon/shutdown", server.handleDaemonShutdown)
	apiMux.HandleFunc("/api/health", server.handleHealth)
	apiMux.HandleFunc("/api/presets", server.handlePresets)
	apiMux.HandleFunc("/api/environments", server.handleEnvironments)
	apiMux.HandleFunc("/api/environments/", server.handleEnvironment)

	apiHandler := withLocalCORS(apiMux)
	if server.staticDir == "" {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if strings.HasPrefix(request.URL.Path, "/api/") {
				apiHandler.ServeHTTP(writer, request)
				return
			}

			if request.URL.Path == "/" {
				writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
				_, _ = writer.Write([]byte("ELK-Local API is running. Build the web UI or run Vite to use the dashboard.\n"))
				return
			}

			http.NotFound(writer, request)
		})
	}

	fileServer := http.FileServer(http.Dir(server.staticDir))
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.HasPrefix(request.URL.Path, "/api/") {
			apiHandler.ServeHTTP(writer, request)
			return
		}

		cleanPath := filepath.Clean(strings.TrimPrefix(request.URL.Path, "/"))
		candidate := filepath.Join(server.staticDir, cleanPath)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(writer, request)
			return
		}

		http.ServeFile(writer, request, filepath.Join(server.staticDir, "index.html"))
	})
}

func (server *Server) StaticDir() string {
	if server == nil {
		return ""
	}

	return server.staticDir
}

func (server *Server) ListenAndServe(host string, port int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return err
	}

	return server.Listen(listener)
}

func (server *Server) Listen(listener net.Listener) error {
	httpServer := &http.Server{
		Handler:           server.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	server.serverMu.Lock()
	server.httpServer = httpServer
	server.serverMu.Unlock()
	defer func() {
		server.serverMu.Lock()
		server.httpServer = nil
		server.serverMu.Unlock()
	}()

	signalContext, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stopSignals()

	go func() {
		<-signalContext.Done()
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownContext)
	}()

	log.Printf("ELK-Local API listening on http://%s", listener.Addr().String())
	if server.staticDir != "" {
		log.Printf("Serving web UI assets from %s", server.staticDir)
	}

	err := httpServer.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}

func (server *Server) Shutdown(ctx context.Context) error {
	server.serverMu.RLock()
	httpServer := server.httpServer
	server.serverMu.RUnlock()
	if httpServer == nil {
		return nil
	}

	return httpServer.Shutdown(ctx)
}

func (server *Server) handleDaemonStatus(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeMethodNotAllowed(writer, http.MethodGet)
		return
	}

	writeJSON(writer, http.StatusOK, map[string]any{
		"ok":          true,
		"projectRoot": server.projectRoot,
		"staticDir":   server.staticDir,
	})
}

func (server *Server) handleDaemonShutdown(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeMethodNotAllowed(writer, http.MethodPost)
		return
	}

	writeJSON(writer, http.StatusAccepted, map[string]any{"ok": true})

	go func() {
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownContext)
	}()
}

func (server *Server) handleHealth(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeMethodNotAllowed(writer, http.MethodGet)
		return
	}

	writeJSON(writer, http.StatusOK, map[string]any{
		"ok":          true,
		"projectRoot": server.projectRoot,
	})
}

func (server *Server) handlePresets(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeMethodNotAllowed(writer, http.MethodGet)
		return
	}

	writeJSON(writer, http.StatusOK, map[string]any{
		"presets": toPresetViews(environment.ListPresets()),
	})
}

func (server *Server) handleEnvironments(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		defaultProjectRootBase, err := defaultCreateProjectRootBase()
		if err != nil {
			writeError(writer, http.StatusInternalServerError, err)
			return
		}

		manifests, err := environment.ListManifests(server.projectRoot)
		if err != nil {
			writeError(writer, http.StatusInternalServerError, err)
			return
		}

		environments := make([]EnvironmentView, 0, len(manifests))
		for _, manifest := range manifests {
			environmentView, err := server.inspectManifest(manifest)
			if err != nil {
				writeError(writer, http.StatusInternalServerError, err)
				return
			}

			environments = append(environments, environmentView)
		}

		writeJSON(writer, http.StatusOK, environmentsResponse{
			ProjectRoot:            server.projectRoot,
			DefaultProjectRootBase: defaultProjectRootBase,
			Environments:           environments,
			PresetOptions:          toPresetViews(environment.ListPresets()),
		})
	case http.MethodPost:
		var payload createEnvironmentRequest
		if err := decodeJSON(request, &payload); err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}

		created, err := environment.Create(environment.CreateOptions{
			Name:                 payload.Name,
			Preset:               payload.Preset,
			ApplicationVersion:   payload.ApplicationVersion,
			ProjectRoot:          payload.ProjectRoot,
			PHPVersion:           payload.PHPVersion,
			WebServer:            payload.WebServer,
			DatabaseEngine:       payload.DatabaseEngine,
			DatabaseVersion:      payload.DatabaseVersion,
			DatabaseName:         payload.DatabaseName,
			DatabaseUser:         payload.DatabaseUser,
			DatabasePassword:     payload.DatabasePassword,
			DatabaseRootPassword: payload.DatabaseRootPassword,
			AdminerEnabled:       payload.AdminerEnabled,
			MailpitEnabled:       payload.MailpitEnabled,
			XdebugEnabled:        payload.XdebugEnabled,
			Force:                payload.Force,
			Installer:            server.installer,
		})
		if err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}

		environmentView, err := server.inspectManifest(created.Manifest)
		if err != nil {
			writeError(writer, http.StatusInternalServerError, err)
			return
		}

		writeJSON(writer, http.StatusCreated, environmentResponse{Environment: environmentView})
	default:
		writeMethodNotAllowed(writer, http.MethodGet, http.MethodPost)
	}
}

func defaultCreateProjectRootBase() (string, error) {
	settings, configured, err := config.Load()
	if err != nil {
		return "", err
	}

	if configured {
		return settings.EnvironmentsDir, nil
	}

	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve current working directory: %w", err)
	}

	return filepath.Clean(currentDir), nil
}

func (server *Server) handleEnvironment(writer http.ResponseWriter, request *http.Request) {
	path := strings.TrimPrefix(request.URL.Path, "/api/environments/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(writer, request)
		return
	}

	name := parts[0]
	manifestPath, err := environment.ResolveManifestPath(server.projectRoot, name)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}

	manifest, err := environment.LoadManifest(manifestPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(writer, request)
			return
		}

		writeError(writer, http.StatusInternalServerError, err)
		return
	}

	if len(parts) == 1 {
		switch request.Method {
		case http.MethodGet:
			environmentView, err := server.inspectManifest(manifest)
			if err != nil {
				writeError(writer, http.StatusInternalServerError, err)
				return
			}

			writeJSON(writer, http.StatusOK, environmentResponse{Environment: environmentView})
		case http.MethodDelete:
			if _, err := server.lifecycle.Destroy(manifest); err != nil {
				writeError(writer, http.StatusInternalServerError, fmt.Errorf("destroy containers: %w", err))
				return
			}

			result, err := environment.Delete(server.projectRoot, manifest.Name)
			if err != nil {
				writeError(writer, http.StatusBadRequest, err)
				return
			}

			writeJSON(writer, http.StatusOK, deleteEnvironmentResponse{
				Name:                manifest.Name,
				RemovedProjectFiles: result.RemovedProjectFiles,
				RemovedBackups:      result.RemovedBackups,
			})
		default:
			writeMethodNotAllowed(writer, http.MethodGet, http.MethodDelete)
		}
		return
	}

	if parts[1] == "backups" {
		server.handleBackups(writer, request, manifest, parts[2:])
		return
	}

	if len(parts) == 3 && parts[1] == "actions" && request.Method == http.MethodPost {
		output, err := server.runAction(parts[2], manifest)
		if err != nil {
			writeError(writer, http.StatusBadRequest, fmt.Errorf("%w\n%s", err, strings.TrimSpace(output)))
			return
		}

		environmentView, inspectErr := server.inspectManifest(manifest)
		if inspectErr != nil {
			writeError(writer, http.StatusInternalServerError, inspectErr)
			return
		}

		writeJSON(writer, http.StatusOK, environmentResponse{Environment: environmentView, Output: output})
		return
	}

	writeMethodNotAllowed(writer, http.MethodGet, http.MethodPost)
}

func (server *Server) handleBackups(writer http.ResponseWriter, request *http.Request, manifest environment.Manifest, tail []string) {
	if len(tail) == 0 {
		if request.Method != http.MethodGet {
			writeMethodNotAllowed(writer, http.MethodGet)
			return
		}

		backups, err := server.listBackups(manifest)
		if err != nil {
			writeError(writer, http.StatusInternalServerError, err)
			return
		}

		writeJSON(writer, http.StatusOK, backupInventoryResponse{
			EnvironmentName: manifest.Name,
			Backups:         backups,
		})
		return
	}

	if len(tail) == 1 && request.Method == http.MethodDelete {
		server.handleManagedBackupDelete(writer, manifest, tail[0])
		return
	}

	if len(tail) == 2 && tail[1] == "download" {
		if request.Method != http.MethodGet {
			writeMethodNotAllowed(writer, http.MethodGet)
			return
		}

		server.handleManagedBackupDownload(writer, request, manifest, tail[0])
		return
	}

	if len(tail) == 3 && tail[1] == "actions" && tail[2] == "open-folder" {
		if request.Method != http.MethodPost {
			writeMethodNotAllowed(writer, http.MethodPost)
			return
		}

		server.handleManagedBackupOpenFolder(writer, manifest, tail[0])
		return
	}

	if len(tail) != 1 || request.Method != http.MethodPost {
		writeMethodNotAllowed(writer, http.MethodGet, http.MethodPost, http.MethodDelete)
		return
	}

	switch tail[0] {
	case "create", "export":
		var payload createBackupRequest
		if err := decodeJSON(request, &payload); err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}

		artifact, err := server.backup.Backup(manifest, environment.BackupCreateOptions{
			OutputPath:          payload.OutputPath,
			IncludeProjectFiles: payload.IncludeProjectFiles,
			Force:               payload.Force,
		})
		if err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}

		backupsAfter, err := server.listBackups(manifest)
		if err != nil {
			writeError(writer, http.StatusInternalServerError, err)
			return
		}

		backupView, err := toBackupViewFromArtifact(artifact)
		if err != nil {
			writeError(writer, http.StatusInternalServerError, err)
			return
		}

		message := "Managed backup created"
		statusCode := http.StatusCreated
		if tail[0] == "export" {
			message = "Backup exported"
			statusCode = http.StatusOK
		}

		writeJSON(writer, statusCode, backupActionResponse{
			EnvironmentName: manifest.Name,
			Backup:          backupView,
			Backups:         backupsAfter,
			Message:         message,
		})
	case "import":
		var payload importBackupRequest
		if err := decodeJSON(request, &payload); err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}

		artifact, err := server.backup.Import(manifest, payload.ArchivePath, payload.Force)
		if err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}

		backupsAfter, err := server.listBackups(manifest)
		if err != nil {
			writeError(writer, http.StatusInternalServerError, err)
			return
		}

		backupView, err := toBackupViewFromArtifact(artifact)
		if err != nil {
			writeError(writer, http.StatusInternalServerError, err)
			return
		}

		writeJSON(writer, http.StatusCreated, backupActionResponse{
			EnvironmentName: manifest.Name,
			Backup:          backupView,
			Backups:         backupsAfter,
			Message:         "Backup imported into managed inventory",
		})
	case "restore":
		backupsBefore, err := server.listBackups(manifest)
		if err != nil {
			writeError(writer, http.StatusInternalServerError, err)
			return
		}

		var payload restoreBackupRequest
		if err := decodeJSON(request, &payload); err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}

		result, err := server.backup.Restore(manifest, environment.RestoreOptions{
			ArchivePath:         payload.ArchivePath,
			RestoreProjectFiles: payload.RestoreProjectFiles,
			Force:               payload.Force,
		})
		if err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}

		backupsAfter, err := server.listBackups(manifest)
		if err != nil {
			writeError(writer, http.StatusInternalServerError, err)
			return
		}

		backupView, err := toBackupView(result.Path, result.Metadata)
		if err != nil {
			writeError(writer, http.StatusInternalServerError, err)
			return
		}

		writeJSON(writer, http.StatusOK, backupActionResponse{
			EnvironmentName:      manifest.Name,
			Backup:               backupView,
			Backups:              chooseBackupsAfterRestore(backupsAfter, backupsBefore),
			RestoredProjectFiles: result.RestoredProjectFiles,
			Message:              "Backup restored",
		})
	default:
		writeMethodNotAllowed(writer, http.MethodGet, http.MethodPost)
	}
}

func (server *Server) handleManagedBackupDelete(writer http.ResponseWriter, manifest environment.Manifest, encodedArchiveName string) {
	archiveName, err := decodeBackupPathSegment(encodedArchiveName)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}

	backupView, err := server.lookupManagedBackup(manifest, archiveName)
	if err != nil {
		writeManagedBackupError(writer, err)
		return
	}

	if err := server.backup.DeleteManagedArchive(manifest, archiveName); err != nil {
		writeManagedBackupError(writer, err)
		return
	}

	backupsAfter, err := server.listBackups(manifest)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}

	writeJSON(writer, http.StatusOK, backupActionResponse{
		EnvironmentName: manifest.Name,
		Backup:          backupView,
		Backups:         backupsAfter,
		Message:         "Managed backup deleted",
	})
}

func (server *Server) handleManagedBackupDownload(writer http.ResponseWriter, request *http.Request, manifest environment.Manifest, encodedArchiveName string) {
	archiveName, err := decodeBackupPathSegment(encodedArchiveName)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}

	archivePath, err := server.backup.ResolveManagedArchive(manifest, archiveName)
	if err != nil {
		writeManagedBackupError(writer, err)
		return
	}

	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(archivePath)))
	writer.Header().Set("Content-Type", "application/gzip")
	http.ServeFile(writer, request, archivePath)
}

func (server *Server) handleManagedBackupOpenFolder(writer http.ResponseWriter, manifest environment.Manifest, encodedArchiveName string) {
	archiveName, err := decodeBackupPathSegment(encodedArchiveName)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}

	backupView, err := server.lookupManagedBackup(manifest, archiveName)
	if err != nil {
		writeManagedBackupError(writer, err)
		return
	}

	archivePath, err := server.backup.ResolveManagedArchive(manifest, archiveName)
	if err != nil {
		writeManagedBackupError(writer, err)
		return
	}

	if server.explorer == nil {
		writeError(writer, http.StatusInternalServerError, fmt.Errorf("file explorer integration is not configured"))
		return
	}

	folderPath := filepath.Dir(archivePath)
	if err := server.explorer.OpenFolder(folderPath); err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}

	writeJSON(writer, http.StatusOK, backupActionResponse{
		EnvironmentName: manifest.Name,
		Backup:          backupView,
		Message:         fmt.Sprintf("Opening %s in the system file explorer", folderPath),
	})
}

func (server *Server) runAction(action string, manifest environment.Manifest) (string, error) {
	switch action {
	case "start":
		return server.lifecycle.Start(manifest)
	case "stop":
		return server.lifecycle.Stop(manifest)
	case "destroy":
		return server.lifecycle.Destroy(manifest)
	case "open-editor":
		if server.editor == nil {
			return "", fmt.Errorf("VS Code integration is not configured")
		}
		if err := server.editor.OpenFolder(manifest.Project.Root); err != nil {
			return "", err
		}
		return fmt.Sprintf("Opening %s in VS Code", manifest.Project.Root), nil
	default:
		return "", fmt.Errorf("unknown action %q", action)
	}
}

func (server *Server) inspectManifest(manifest environment.Manifest) (EnvironmentView, error) {
	info, err := os.Stat(manifest.Compose.File)
	updatedAt := ""
	if err == nil {
		updatedAt = info.ModTime().UTC().Format(time.RFC3339)
	}

	status := StatusView{State: "stopped", Containers: nil}
	output, statusErr := server.lifecycle.Status(manifest)
	if statusErr == nil {
		containers, parseErr := parseComposeStatus(output)
		if parseErr != nil {
			status.State = "unknown"
			status.Error = parseErr.Error()
			status.RawOutput = output
		} else {
			status.Containers = toContainerViews(containers)
			status.State = classifyContainerState(containers)
		}
	} else if strings.TrimSpace(output) != "" {
		containers, parseErr := parseComposeStatus(output)
		if parseErr == nil {
			status.Containers = toContainerViews(containers)
			status.State = classifyContainerState(containers)
		} else {
			status.State = "unknown"
			status.RawOutput = output
		}
		status.Error = statusErr.Error()
	} else {
		status.State = "stopped"
		status.Error = statusErr.Error()
	}

	return EnvironmentView{
		Name:         manifest.Name,
		Preset:       manifest.Preset,
		Application:  ApplicationView{Name: manifest.Application.Name, Version: manifest.Application.Version},
		ProjectType:  manifest.Project.Type,
		ProjectRoot:  manifest.Project.Root,
		ComposePath:  manifest.Compose.File,
		StoragePath:  manifest.Storage.BasePath,
		PHPVersion:   manifest.Runtime.PHPVersion,
		WebServer:    manifest.Runtime.WebServer,
		ManifestPath: filepath.Join(manifest.Storage.BasePath, "environment.yaml"),
		UpdatedAt:    updatedAt,
		Database: DatabaseView{
			Engine:   manifest.Runtime.Database.Engine,
			Version:  manifest.Runtime.Database.Version,
			Name:     manifest.Runtime.Database.Name,
			User:     manifest.Runtime.Database.User,
			Host:     "127.0.0.1",
			Port:     manifest.Network.DatabasePort,
			RootUser: "root",
		},
		Tooling: ToolingView{
			Adminer: ToolToggleView{Enabled: manifest.Tooling.Adminer.Enabled, Port: manifest.Tooling.Adminer.Port},
			Mailpit: ToolToggleView{Enabled: manifest.Tooling.Mailpit.Enabled, Port: manifest.Tooling.Mailpit.HTTPPort, SMTPPort: manifest.Tooling.Mailpit.SMTPPort},
			Xdebug:  XdebugView{Enabled: manifest.Tooling.Xdebug.Enabled, ClientHost: manifest.Tooling.Xdebug.ClientHost, ClientPort: manifest.Tooling.Xdebug.ClientPort},
		},
		Network: NetworkView{HTTPPort: manifest.Network.HTTPPort, DatabasePort: manifest.Network.DatabasePort},
		URLs:    buildURLs(manifest),
		Status:  status,
	}, nil
}

func buildURLs(manifest environment.Manifest) URLView {
	urls := URLView{
		App:      fmt.Sprintf("http://127.0.0.1:%d", manifest.Network.HTTPPort),
		Database: fmt.Sprintf("127.0.0.1:%d", manifest.Network.DatabasePort),
	}

	if manifest.Tooling.Adminer.Enabled {
		urls.Adminer = fmt.Sprintf("http://127.0.0.1:%d", manifest.Tooling.Adminer.Port)
	}

	if manifest.Tooling.Mailpit.Enabled {
		urls.Mailpit = fmt.Sprintf("http://127.0.0.1:%d", manifest.Tooling.Mailpit.HTTPPort)
		urls.SMTP = fmt.Sprintf("127.0.0.1:%d", manifest.Tooling.Mailpit.SMTPPort)
	}

	return urls
}

func (server *Server) listBackups(manifest environment.Manifest) ([]BackupView, error) {
	items, err := server.backup.List(manifest)
	if err != nil {
		return nil, err
	}

	views := make([]BackupView, 0, len(items))
	for _, item := range items {
		views = append(views, BackupView{
			FileName:             item.FileName,
			Path:                 item.Path,
			SizeBytes:            item.SizeBytes,
			CreatedAt:            item.CreatedAt,
			EnvironmentName:      item.EnvironmentName,
			DatabaseName:         item.DatabaseName,
			IncludesProjectFiles: item.IncludesProjectFiles,
			Error:                item.Error,
		})
	}

	return views, nil
}

func (server *Server) lookupManagedBackup(manifest environment.Manifest, archiveName string) (BackupView, error) {
	backups, err := server.listBackups(manifest)
	if err != nil {
		return BackupView{}, err
	}

	for _, backup := range backups {
		if backup.FileName == archiveName {
			return backup, nil
		}
	}

	if _, err := server.backup.ResolveManagedArchive(manifest, archiveName); err != nil {
		return BackupView{}, err
	}

	return BackupView{}, fmt.Errorf("managed backup archive %s not found", archiveName)
}

func decodeBackupPathSegment(segment string) (string, error) {
	decodedSegment, err := url.PathUnescape(segment)
	if err != nil {
		return "", fmt.Errorf("decode backup file name: %w", err)
	}

	trimmedSegment := strings.TrimSpace(decodedSegment)
	if trimmedSegment == "" {
		return "", fmt.Errorf("backup file name is required")
	}

	return trimmedSegment, nil
}

func writeManagedBackupError(writer http.ResponseWriter, err error) {
	statusCode := http.StatusBadRequest
	if errors.Is(err, os.ErrNotExist) {
		statusCode = http.StatusNotFound
	}

	writeError(writer, statusCode, err)
}

func toBackupViewFromArtifact(artifact environment.BackupArtifact) (BackupView, error) {
	return toBackupView(artifact.Path, artifact.Metadata)
}

func toBackupView(path string, metadata environment.BackupMetadata) (BackupView, error) {
	info, err := os.Stat(path)
	if err != nil {
		return BackupView{}, fmt.Errorf("stat backup archive %s: %w", path, err)
	}

	return BackupView{
		FileName:             filepath.Base(path),
		Path:                 path,
		SizeBytes:            info.Size(),
		CreatedAt:            firstNonEmpty(metadata.CreatedAt, info.ModTime().UTC().Format(time.RFC3339)),
		EnvironmentName:      metadata.EnvironmentName,
		DatabaseName:         metadata.DatabaseName,
		IncludesProjectFiles: metadata.IncludesProjectFiles,
	}, nil
}

func chooseBackupsAfterRestore(backupsAfter []BackupView, backupsBefore []BackupView) []BackupView {
	if len(backupsAfter) > 0 {
		return backupsAfter
	}

	return backupsBefore
}

func parseComposeStatus(output string) ([]composeContainer, error) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" || trimmed == "[]" {
		return nil, nil
	}

	containers := []composeContainer{}
	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal([]byte(trimmed), &containers); err != nil {
			return nil, fmt.Errorf("decode compose status array: %w", err)
		}

		sort.Slice(containers, func(left int, right int) bool {
			return containers[left].Service < containers[right].Service
		})
		return containers, nil
	}

	for _, line := range strings.Split(trimmed, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var container composeContainer
		if err := json.Unmarshal([]byte(line), &container); err != nil {
			return nil, fmt.Errorf("decode compose status line: %w", err)
		}

		containers = append(containers, container)
	}

	sort.Slice(containers, func(left int, right int) bool {
		return containers[left].Service < containers[right].Service
	})

	return containers, nil
}

func toContainerViews(containers []composeContainer) []ContainerView {
	views := make([]ContainerView, 0, len(containers))
	for _, container := range containers {
		ports := make([]string, 0, len(container.Publishers))
		seenPorts := make(map[string]struct{}, len(container.Publishers))
		for _, publisher := range container.Publishers {
			if publisher.PublishedPort == 0 {
				continue
			}

			portMapping := fmt.Sprintf("%d->%d/%s", publisher.PublishedPort, publisher.TargetPort, strings.ToLower(firstNonEmpty(publisher.Protocol, "tcp")))
			if _, seen := seenPorts[portMapping]; seen {
				continue
			}

			seenPorts[portMapping] = struct{}{}
			ports = append(ports, portMapping)
		}

		views = append(views, ContainerView{
			Name:           container.Name,
			Service:        container.Service,
			State:          container.State,
			Health:         container.Health,
			PublishedPorts: ports,
		})
	}

	return views
}

func classifyContainerState(containers []composeContainer) string {
	if len(containers) == 0 {
		return "stopped"
	}

	runningCount := 0
	for _, container := range containers {
		state := strings.ToLower(strings.TrimSpace(container.State))
		if state == "running" {
			runningCount++
		}
	}

	if runningCount == len(containers) {
		return "running"
	}

	if runningCount == 0 {
		return "stopped"
	}

	return "partial"
}

func decodeJSON(request *http.Request, target any) error {
	defer request.Body.Close()

	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}

	return nil
}

func writeJSON(writer http.ResponseWriter, statusCode int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(statusCode)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeError(writer http.ResponseWriter, statusCode int, err error) {
	writeJSON(writer, statusCode, errorResponse{Error: strings.TrimSpace(err.Error())})
}

func writeMethodNotAllowed(writer http.ResponseWriter, methods ...string) {
	writer.Header().Set("Allow", strings.Join(methods, ", "))
	writeJSON(writer, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
}

func withLocalCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		origin := request.Header.Get("Origin")
		if strings.HasPrefix(origin, "http://127.0.0.1:") || strings.HasPrefix(origin, "http://localhost:") {
			writer.Header().Set("Access-Control-Allow-Origin", origin)
			writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			writer.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,OPTIONS")
		}

		if request.Method == http.MethodOptions {
			writer.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(writer, request)
	})
}

func resolveProjectRoot(projectRoot string) (string, error) {
	resolvedPath, err := config.ResolveRegistryRoot(projectRoot)
	if err != nil {
		return "", err
	}

	return filepath.Clean(resolvedPath), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}

	return ""
}

func toPresetViews(presets []environment.Preset) []PresetView {
	views := make([]PresetView, 0, len(presets))
	for _, preset := range presets {
		views = append(views, PresetView{
			Name:              preset.Name,
			Description:       preset.Description,
			ApplicationName:   preset.ApplicationName,
			DefaultAppVersion: preset.DefaultAppVersion,
			AppVersionHint:    preset.AppVersionHint,
			ProjectType:       preset.ProjectType,
			PHPVersion:        preset.PHPVersion,
			WebServer:         preset.WebServer,
			DatabaseEngine:    preset.DatabaseEngine,
			DatabaseVersion:   preset.DatabaseVersion,
		})
	}

	return views
}
