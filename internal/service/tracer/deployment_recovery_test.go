package tracer

import (
	"OmniView/assets"
	"OmniView/internal/core/domain"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"
)

type recoveryConfig struct {
	stubConfigRepository
	hash   string
	writes int
}

func (r *recoveryConfig) GetTracerPackageVersion() (string, error) { return r.hash, nil }
func (r *recoveryConfig) SetTracerPackageVersion(hash string) error {
	r.hash = hash
	r.writes++
	return nil
}

type recoveryDatabase struct {
	stubDatabaseRepository
	packageExists      bool
	packageBodyInvalid bool
	queueExists        bool
	initializations    int
	initializationErr  error
	deployments        int
	deploymentErr      error
}

func (r *recoveryDatabase) PackageExists(context.Context, string) (bool, error) {
	return r.packageExists && !r.packageBodyInvalid, nil
}

func (r *recoveryDatabase) DeployFile(context.Context, string) error {
	r.deployments++
	if r.deploymentErr != nil {
		return r.deploymentErr
	}
	r.packageExists = true
	r.packageBodyInvalid = false
	return nil
}

func (r *recoveryDatabase) ExecuteStatement(context.Context, string) error {
	r.initializations++
	if r.initializationErr != nil {
		return r.initializationErr
	}
	r.queueExists = true
	return nil
}

func (r *recoveryDatabase) RegisterNewSubscriber(context.Context, domain.Subscriber) error {
	if !r.queueExists {
		return errors.New("ORA-24010: QUEUE IFSAPP.OMNI_TRACER_QUEUE does not exist")
	}
	return nil
}

func TestDeployAndCheck_RecoversAfterInitializationFailure(t *testing.T) {
	db := &recoveryDatabase{initializationErr: errors.New("ORA-01653: unable to extend table in tablespace")}
	service := &TracerService{db: db, bolt: &recoveryConfig{}}
	ctx := context.Background()
	if err := service.DeployAndCheck(ctx); err == nil {
		t.Fatal("expected interrupted initialization to block first startup")
	}
	db.initializationErr = nil
	// A fresh service models the next login; deployed objects and local cache survive.
	service = &TracerService{db: db, bolt: service.bolt}
	if err := service.DeployAndCheck(ctx); err != nil {
		t.Fatalf("retry after tablespace recovery: %v", err)
	}
	if err := db.RegisterNewSubscriber(ctx, *newTestSubscriber(t)); err != nil {
		t.Fatalf("second login reports deployment success but registration is still blocked: %v", err)
	}
}

func tracerPackageHash(t *testing.T) string {
	t.Helper()
	sql, err := assets.GetSQLFile("Omni_Tracer.sql")
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(sql))
}

func TestDeployAndCheck_DoesNotCacheFailedInitialization(t *testing.T) {
	for _, hash := range []string{"", "previous-successful-version"} {
		t.Run(hash, func(t *testing.T) {
			failure := errors.New("ORA-03113: end-of-file on communication channel")
			db := &recoveryDatabase{initializationErr: failure}
			config := &recoveryConfig{hash: hash}
			service := &TracerService{db: db, bolt: config}
			if err := service.DeployAndCheck(context.Background()); !errors.Is(err, failure) {
				t.Fatalf("expected initialization failure, got %v", err)
			}
			if config.hash != hash || config.writes != 0 {
				t.Fatal("failed readiness must not advance the deployment cache")
			}
		})
	}
}

func TestDeployAndCheck_ReconcilesCachedDeployment(t *testing.T) {
	for _, queueExists := range []bool{false, true} {
		t.Run(fmt.Sprintf("queueExists=%t", queueExists), func(t *testing.T) {
			db := &recoveryDatabase{packageExists: true, queueExists: queueExists}
			config := &recoveryConfig{hash: tracerPackageHash(t)}
			service := &TracerService{db: db, bolt: config}
			if err := service.DeployAndCheck(context.Background()); err != nil {
				t.Fatal(err)
			}
			if db.initializations != 1 || !db.queueExists {
				t.Fatal("cached package must still reconcile queue readiness")
			}
			if db.deployments != 0 || config.writes != 0 {
				t.Fatal("healthy cached package must not be redeployed or rewrite its hash")
			}
		})
	}
}

func TestDeployAndCheck_PersistentFailureStaysBlocked(t *testing.T) {
	failure := errors.New("ORA-01653: unable to extend table in tablespace")
	db := &recoveryDatabase{packageExists: true, initializationErr: failure}
	config := &recoveryConfig{hash: tracerPackageHash(t)}
	service := &TracerService{db: db, bolt: config}
	for attempt := 0; attempt < 2; attempt++ {
		if err := service.DeployAndCheck(context.Background()); !errors.Is(err, failure) {
			t.Fatalf("attempt %d should remain blocked by the real error, got %v", attempt, err)
		}
	}
	if config.writes != 0 {
		t.Fatal("persistent failure must not update the cache")
	}
}

func TestDeployAndCheck_RecoversInterruptedPackageDeployment(t *testing.T) {
	failure := errors.New("ORA-03113: end-of-file on communication channel")
	db := &recoveryDatabase{deploymentErr: failure}
	config := &recoveryConfig{hash: tracerPackageHash(t)}
	service := &TracerService{db: db, bolt: config}
	if err := service.DeployAndCheck(context.Background()); !errors.Is(err, failure) {
		t.Fatalf("expected deployment failure, got %v", err)
	}
	if config.writes != 0 || db.initializations != 0 {
		t.Fatal("failed deployment must not initialize or advance the cache")
	}
	db.deploymentErr = nil
	if err := service.DeployAndCheck(context.Background()); err != nil {
		t.Fatal(err)
	}
	if db.deployments != 2 || db.initializations != 1 || config.writes != 1 {
		t.Fatal("invalid or missing package must redeploy despite the matching hash")
	}
}

func TestDeployAndCheck_RepairsInvalidBodyWithCachedHash(t *testing.T) {
	db := &recoveryDatabase{packageExists: true, packageBodyInvalid: true}
	config := &recoveryConfig{hash: tracerPackageHash(t)}
	service := &TracerService{db: db, bolt: config}
	ctx := context.Background()

	if err := service.DeployAndCheck(ctx); err != nil {
		t.Fatalf("recover partial deployment with matching cached hash: %v", err)
	}
	if db.deployments != 1 || db.initializations != 1 || config.writes != 1 {
		t.Fatal("valid specification and invalid body must redeploy and initialize despite the cached hash")
	}
	if db.packageBodyInvalid {
		t.Fatal("repair left the package body invalid")
	}
	if err := db.RegisterNewSubscriber(ctx, *newTestSubscriber(t)); err != nil {
		t.Fatalf("subscriber registration after partial deployment recovery: %v", err)
	}
}
