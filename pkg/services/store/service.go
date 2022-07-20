package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/grafana/grafana/pkg/infra/filestorage"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/registry"
	ac "github.com/grafana/grafana/pkg/services/accesscontrol"
	"github.com/grafana/grafana/pkg/services/featuremgmt"
	"github.com/grafana/grafana/pkg/services/sqlstore"
	"github.com/grafana/grafana/pkg/setting"
)

var grafanaStorageLogger = log.New("grafanaStorageLogger")

var ErrUnsupportedStorage = errors.New("storage does not support this operation")
var ErrUploadInternalError = errors.New("upload internal error")
var ErrValidationFailed = errors.New("request validation failed")
var ErrFileAlreadyExists = errors.New("file exists")
var ErrStorageNotFound = errors.New("storage not found")
var ErrAccessDenied = errors.New("access denied")

const RootPublicStatic = "public-static"
const RootResources = "resources"
const RootDevenv = "devenv"
const RootSystem = "system"

const SystemBrandingStorage = "system/branding"

const MAX_UPLOAD_SIZE = 1 * 1024 * 1024 // 3MB

type DeleteFolderCmd struct {
	Path  string `json:"path"`
	Force bool   `json:"force"`
}

type CreateFolderCmd struct {
	Path string `json:"path"`
}

type StorageService interface {
	registry.BackgroundService

	// List folder contents
	List(ctx context.Context, user *models.SignedInUser, path string) (*StorageListFrame, error)

	// Read raw file contents out of the store
	Read(ctx context.Context, user *models.SignedInUser, path string) (*filestorage.File, error)

	Upload(ctx context.Context, user *models.SignedInUser, req *UploadRequest) error

	Delete(ctx context.Context, user *models.SignedInUser, path string) error

	DeleteFolder(ctx context.Context, user *models.SignedInUser, cmd *DeleteFolderCmd) error

	CreateFolder(ctx context.Context, user *models.SignedInUser, cmd *CreateFolderCmd) error

	validateUploadRequest(ctx context.Context, user *models.SignedInUser, req *UploadRequest, storagePath string) validationResult

	// sanitizeUploadRequest sanitizes the upload request and converts it into a command accepted by the FileStorage API
	sanitizeUploadRequest(ctx context.Context, user *models.SignedInUser, req *UploadRequest, storagePath string) (*filestorage.UpsertFileCommand, error)
}

type storageServiceConfig struct {
	allowUnsanitizedSvgUpload bool
}

type standardStorageService struct {
	sql         *sqlstore.SQLStore
	tree        *nestedTree
	cfg         storageServiceConfig
	authService storageAuthService
}

func ProvideService(sql *sqlstore.SQLStore, features featuremgmt.FeatureToggles, cfg *setting.Cfg) StorageService {
	globalRoots := []storageRuntime{
		newDiskStorage(RootPublicStatic, "Public static files", &StorageLocalDiskConfig{
			Path: cfg.StaticRootPath,
			Roots: []string{
				"/testdata/",
				"/img/",
				"/gazetteer/",
				"/maps/",
			},
		}).setReadOnly(true).setBuiltin(true).
			setDescription("Access files from the static public files"),
	}

	// Development dashboards
	if setting.Env != setting.Prod {
		devenv := filepath.Join(cfg.StaticRootPath, "..", "devenv")
		if _, err := os.Stat(devenv); !os.IsNotExist(err) {
			// path/to/whatever exists
			s := newDiskStorage(RootDevenv, "Development Environment", &StorageLocalDiskConfig{
				Path: devenv,
				Roots: []string{
					"/dev-dashboards/",
				},
			}).setReadOnly(false).setDescription("Explore files within the developer environment directly")
			globalRoots = append(globalRoots, s)
		}
	}

	initializeOrgStorages := func(orgId int64) []storageRuntime {
		storages := make([]storageRuntime, 0)

		// Custom upload files
		storages = append(storages,
			newSQLStorage(RootResources,
				"Resources",
				&StorageSQLConfig{orgId: orgId}, sql).
				setBuiltin(true).
				setDescription("Upload custom resource files"))

		// System settings
		storages = append(storages,
			newSQLStorage(RootSystem,
				"System",
				&StorageSQLConfig{orgId: orgId},
				sql,
			).setBuiltin(true).setDescription("Grafana system storage"))

		return storages
	}

	authService := newStaticStorageAuthService(func(ctx context.Context, user *models.SignedInUser, storageName string) map[string]filestorage.PathFilter {
		if user == nil || !user.IsGrafanaAdmin {
			return nil
		}

		switch storageName {
		case RootPublicStatic:
			return map[string]filestorage.PathFilter{
				ActionFilesRead:   allowAllPathFilter,
				ActionFilesWrite:  denyAllPathFilter,
				ActionFilesDelete: denyAllPathFilter,
			}
		case RootDevenv:
			return map[string]filestorage.PathFilter{
				ActionFilesRead:   allowAllPathFilter,
				ActionFilesWrite:  denyAllPathFilter,
				ActionFilesDelete: denyAllPathFilter,
			}
		case RootResources:
			return map[string]filestorage.PathFilter{
				ActionFilesRead:   allowAllPathFilter,
				ActionFilesWrite:  allowAllPathFilter,
				ActionFilesDelete: allowAllPathFilter,
			}
		case RootSystem:
			return map[string]filestorage.PathFilter{
				ActionFilesRead:   allowAllPathFilter,
				ActionFilesWrite:  allowAllPathFilter,
				ActionFilesDelete: allowAllPathFilter,
			}
		default:
			return nil
		}
	})

	return newStandardStorageService(sql, globalRoots, initializeOrgStorages, authService)
}

func newStandardStorageService(sql *sqlstore.SQLStore, globalRoots []storageRuntime, initializeOrgStorages func(orgId int64) []storageRuntime, authService storageAuthService) *standardStorageService {
	rootsByOrgId := make(map[int64][]storageRuntime)
	rootsByOrgId[ac.GlobalOrgID] = globalRoots

	res := &nestedTree{
		initializeOrgStorages: initializeOrgStorages,
		rootsByOrgId:          rootsByOrgId,
	}
	res.init()
	return &standardStorageService{
		sql:         sql,
		tree:        res,
		authService: authService,
		cfg: storageServiceConfig{
			allowUnsanitizedSvgUpload: false,
		},
	}
}

func (s *standardStorageService) Run(ctx context.Context) error {
	grafanaStorageLogger.Info("storage starting")
	return nil
}

func getOrgId(user *models.SignedInUser) int64 {
	if user == nil {
		return ac.GlobalOrgID
	}

	return user.OrgId
}

func (s *standardStorageService) List(ctx context.Context, user *models.SignedInUser, path string) (*StorageListFrame, error) {
	guardian := s.authService.newGuardian(ctx, user, getFirstSegment(path))
	return s.tree.ListFolder(ctx, getOrgId(user), path, guardian.getPathFilter(ActionFilesRead))
}

func (s *standardStorageService) Read(ctx context.Context, user *models.SignedInUser, path string) (*filestorage.File, error) {
	guardian := s.authService.newGuardian(ctx, user, getFirstSegment(path))
	if !guardian.canView(path) {
		return nil, ErrAccessDenied
	}
	return s.tree.GetFile(ctx, getOrgId(user), path)
}

type UploadRequest struct {
	Contents           []byte
	MimeType           string // TODO: remove MimeType from the struct once we can infer it from file contents
	Path               string
	CacheControl       string
	ContentDisposition string
	Properties         map[string]string
	EntityType         EntityType

	OverwriteExistingFile bool
}

func (s *standardStorageService) Upload(ctx context.Context, user *models.SignedInUser, req *UploadRequest) error {
	guardian := s.authService.newGuardian(ctx, user, getFirstSegment(req.Path))
	if !guardian.canWrite(req.Path) {
		return ErrAccessDenied
	}

	root, storagePath := s.tree.getRoot(getOrgId(user), req.Path)
	if root == nil {
		return ErrStorageNotFound
	}

	if root.Meta().ReadOnly {
		return ErrUnsupportedStorage
	}

	validationResult := s.validateUploadRequest(ctx, user, req, storagePath)
	if !validationResult.ok {
		grafanaStorageLogger.Warn("file upload validation failed", "filetype", req.MimeType, "path", req.Path, "reason", validationResult.reason)
		return ErrValidationFailed
	}

	upsertCommand, err := s.sanitizeUploadRequest(ctx, user, req, storagePath)
	if err != nil {
		grafanaStorageLogger.Error("failed while sanitizing the upload request", "filetype", req.MimeType, "path", req.Path, "error", err)
		return ErrUploadInternalError
	}

	grafanaStorageLogger.Info("uploading a file", "filetype", req.MimeType, "path", req.Path)

	if !req.OverwriteExistingFile {
		file, err := root.Store().Get(ctx, storagePath)
		if err != nil {
			grafanaStorageLogger.Error("failed while checking file existence", "err", err, "path", req.Path)
			return ErrUploadInternalError
		}

		if file != nil {
			return ErrFileAlreadyExists
		}
	}

	if err := root.Store().Upsert(ctx, upsertCommand); err != nil {
		grafanaStorageLogger.Error("failed while uploading the file", "err", err, "path", req.Path)
		return ErrUploadInternalError
	}

	return nil
}

func (s *standardStorageService) DeleteFolder(ctx context.Context, user *models.SignedInUser, cmd *DeleteFolderCmd) error {
	guardian := s.authService.newGuardian(ctx, user, getFirstSegment(cmd.Path))
	if !guardian.canDelete(cmd.Path) {
		return ErrAccessDenied
	}

	root, storagePath := s.tree.getRoot(getOrgId(user), cmd.Path)
	if root == nil {
		return ErrStorageNotFound
	}

	if root.Meta().ReadOnly {
		return ErrUnsupportedStorage
	}

	if storagePath == "" {
		storagePath = filestorage.Delimiter
	}
	return root.Store().DeleteFolder(ctx, storagePath, &filestorage.DeleteFolderOptions{Force: true, AccessFilter: guardian.getPathFilter(ActionFilesDelete)})
}

func (s *standardStorageService) CreateFolder(ctx context.Context, user *models.SignedInUser, cmd *CreateFolderCmd) error {
	guardian := s.authService.newGuardian(ctx, user, getFirstSegment(cmd.Path))
	if !guardian.canWrite(cmd.Path) {
		return ErrAccessDenied
	}

	root, storagePath := s.tree.getRoot(getOrgId(user), cmd.Path)
	if root == nil {
		return ErrStorageNotFound
	}

	if root.Meta().ReadOnly {
		return ErrUnsupportedStorage
	}

	err := root.Store().CreateFolder(ctx, storagePath)
	if err != nil {
		return err
	}
	return nil
}

func (s *standardStorageService) Delete(ctx context.Context, user *models.SignedInUser, path string) error {
	guardian := s.authService.newGuardian(ctx, user, getFirstSegment(path))
	if !guardian.canDelete(path) {
		return ErrAccessDenied
	}

	root, storagePath := s.tree.getRoot(getOrgId(user), path)
	if root == nil {
		return ErrStorageNotFound
	}

	if root.Meta().ReadOnly {
		return ErrUnsupportedStorage
	}

	err := root.Store().Delete(ctx, storagePath)
	if err != nil {
		return err
	}
	return nil
}
