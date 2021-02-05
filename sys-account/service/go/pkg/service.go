package accountpkg

import (
	"context"
	"github.com/amplify-cms/sys-share/sys-core/service/logging"
	"github.com/amplify-cms/sys/sys-account/service/go/pkg/telemetry"
	grpcAuth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	"google.golang.org/grpc"

	"github.com/amplify-cms/sys-share/sys-account/service/go/pkg"
	coresvc "github.com/amplify-cms/sys-share/sys-core/service/go/pkg"
	sharedBus "github.com/amplify-cms/sys-share/sys-core/service/go/pkg/bus"
	"github.com/amplify-cms/sys/sys-account/service/go"
	"github.com/amplify-cms/sys/sys-account/service/go/pkg/repo"
	"github.com/amplify-cms/sys/sys-core/service/go/pkg/coredb"
	corefile "github.com/amplify-cms/sys/sys-core/service/go/pkg/filesvc/repo"
	coremail "github.com/amplify-cms/sys/sys-core/service/go/pkg/mailer"
)

type SysAccountService struct {
	authInterceptorFunc func(context.Context) (context.Context, error)
	proxyService        *pkg.SysAccountProxyService
	DbProxyService      *coresvc.SysCoreProxyService
	BusProxyService     *coresvc.SysBusProxyService
	MailProxyService    *coresvc.SysEmailProxyService
	AuthRepo            *repo.SysAccountRepo
	BusinessTelemetry   *telemetry.SysAccountMetrics
	AllDBs              *coredb.AllDBService
}

type SysAccountServiceConfig struct {
	store    *coredb.CoreDB
	Cfg      *service.SysAccountConfig
	bus      *sharedBus.CoreBus
	mail     *coremail.MailSvc
	logger   logging.Logger
	fileRepo *corefile.SysFileRepo
	allDbs   *coredb.AllDBService
}

func NewSysAccountServiceConfig(l logging.Logger, filepath string, bus *sharedBus.CoreBus, accountCfg *service.SysAccountConfig) (*SysAccountServiceConfig, error) {
	var err error
	sysAccountLogger := l.WithFields(map[string]interface{}{"service": "sys-account"})

	if filepath != "" {
		accountCfg, err = service.NewConfig(filepath)
		if err != nil {
			return nil, err
		}
	}
	// accounts database
	db, err := coredb.NewCoreDB(l, &accountCfg.SysCoreConfig, nil)
	if err != nil {
		return nil, err
	}

	mailSvc := coremail.NewMailSvc(&accountCfg.MailConfig, l)
	// files database
	fileDb, err := coredb.NewCoreDB(l, &accountCfg.SysFileConfig, nil)
	if err != nil {
		return nil, err
	}
	fileRepo, err := corefile.NewSysFileRepo(fileDb, l)
	if err != nil {
		return nil, err
	}

	allDb := coredb.NewAllDBService()
	sysAccountLogger.Info("registering sys-accounts db & filedb to allDb service")
	allDb.RegisterCoreDB(db)
	allDb.RegisterCoreDB(fileDb)

	sasc := &SysAccountServiceConfig{
		store:    db,
		Cfg:      accountCfg,
		bus:      bus,
		logger:   sysAccountLogger,
		mail:     mailSvc,
		fileRepo: fileRepo,
		allDbs:   allDb,
	}
	return sasc, nil
}

func NewSysAccountService(cfg *SysAccountServiceConfig, domain string) (*SysAccountService, error) {
	cfg.logger.Info("Initializing Sys-Account Service")

	sysAccountMetrics := telemetry.NewSysAccountMetrics(cfg.logger)

	authRepo, err := repo.NewAuthRepo(cfg.logger, cfg.store, cfg.Cfg, cfg.bus, cfg.mail, cfg.fileRepo, domain,
		cfg.Cfg.SuperUserFilePath, sysAccountMetrics)
	if err != nil {
		return nil, err
	}
	sysAccountProxy := pkg.NewSysAccountProxyService(authRepo, authRepo, authRepo)

	dbProxyService := coresvc.NewSysCoreProxyService(cfg.allDbs)
	busProxyService := coresvc.NewSysBusProxyService(cfg.bus)
	mailSvc := coresvc.NewSysMailProxyService(cfg.mail)

	return &SysAccountService{
		authInterceptorFunc: authRepo.DefaultInterceptor,
		proxyService:        sysAccountProxy,
		AuthRepo:            authRepo,
		DbProxyService:      dbProxyService,
		BusProxyService:     busProxyService,
		MailProxyService:    mailSvc,
		BusinessTelemetry:   sysAccountMetrics,
		AllDBs:              cfg.allDbs,
	}, nil
}

func (sas *SysAccountService) InjectInterceptors(unaryItc []grpc.UnaryServerInterceptor, streamItc []grpc.StreamServerInterceptor) ([]grpc.UnaryServerInterceptor, []grpc.StreamServerInterceptor) {
	unaryItc = append(unaryItc, grpcAuth.UnaryServerInterceptor(sas.authInterceptorFunc))
	streamItc = append(streamItc, grpcAuth.StreamServerInterceptor(sas.authInterceptorFunc))
	return unaryItc, streamItc
}

func (sas *SysAccountService) RegisterServices(srv *grpc.Server) {
	sas.proxyService.RegisterSvc(srv)
	sas.DbProxyService.RegisterSvc(srv)
	sas.MailProxyService.RegisterSvc(srv)
}
