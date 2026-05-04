package subscribers

import (
	"OmniView/internal/core/domain"
	"context"
	"errors"
	"strings"
	"testing"
)

type stubDBRepo struct {
	procedureExists     map[string]bool
	procedureExistsErr  error
	deployFileErr       error
	deployFileCallCount int
	deployedSQL         string
	registerErr         error
	registeredConsumers []string
	packageSpecSource   []string
	packageBodySource   []string
	fetchErr            error
}

func (s *stubDBRepo) RegisterNewSubscriber(ctx context.Context, subscriber domain.Subscriber) error {
	s.registeredConsumers = append(s.registeredConsumers, subscriber.ConsumerName())
	return s.registerErr
}

func (s *stubDBRepo) BulkDequeueTracerMessages(ctx context.Context, subscriber domain.Subscriber) ([]string, [][]byte, int, error) {
	return nil, nil, 0, nil
}

func (s *stubDBRepo) CheckQueueDepth(ctx context.Context, subscriberID string, queueTableName string) (int, error) {
	return 0, nil
}

func (s *stubDBRepo) Fetch(ctx context.Context, query string) ([]string, error) {
	if s.fetchErr != nil {
		return nil, s.fetchErr
	}
	if strings.Contains(query, "PACKAGE BODY") {
		return append([]string(nil), s.packageBodySource...), nil
	}
	if strings.Contains(query, "type = 'PACKAGE'") {
		return append([]string(nil), s.packageSpecSource...), nil
	}
	return nil, nil
}

func (s *stubDBRepo) ExecuteStatement(ctx context.Context, query string) error {
	return nil
}

func (s *stubDBRepo) ExecuteWithParams(ctx context.Context, query string, params map[string]interface{}) error {
	return nil
}

func (s *stubDBRepo) FetchWithParams(ctx context.Context, query string, params map[string]interface{}) ([]string, error) {
	return nil, nil
}

func (s *stubDBRepo) PackageExists(ctx context.Context, packageName string) (bool, error) {
	return true, nil
}

func (s *stubDBRepo) ProcedureExists(ctx context.Context, procedureName string) (bool, error) {
	if s.procedureExistsErr != nil {
		return false, s.procedureExistsErr
	}
	return s.procedureExists[procedureName], nil
}

func (s *stubDBRepo) DeployPackages(ctx context.Context, sequences []string, types []string, packageSpec []string, packageBody []string) error {
	return nil
}

func (s *stubDBRepo) DeployFile(ctx context.Context, sqlContent string) error {
	s.deployFileCallCount++
	s.deployedSQL = sqlContent
	return s.deployFileErr
}

func (s *stubDBRepo) Connect(ctx context.Context) error { return nil }
func (s *stubDBRepo) Close(ctx context.Context) error   { return nil }

type stubSubscriberRepo struct {
	list    []domain.Subscriber
	saved   []domain.Subscriber
	saveErr error
}

func (s *stubSubscriberRepo) Save(ctx context.Context, subscriber domain.Subscriber) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.saved = append(s.saved, subscriber)
	if len(s.list) == 0 {
		s.list = []domain.Subscriber{subscriber}
	} else {
		s.list[0] = subscriber
	}
	return nil
}

func (s *stubSubscriberRepo) GetByName(ctx context.Context, name string) (*domain.Subscriber, error) {
	for i := range s.list {
		if s.list[i].Name() == name {
			subscriber := s.list[i]
			return &subscriber, nil
		}
	}
	return nil, domain.ErrSubscriberNotFound
}

func (s *stubSubscriberRepo) List(ctx context.Context) ([]domain.Subscriber, error) {
	return append([]domain.Subscriber(nil), s.list...), nil
}

func (s *stubSubscriberRepo) Exists(ctx context.Context, name string) (bool, error) {
	for _, subscriber := range s.list {
		if subscriber.Name() == name {
			return true, nil
		}
	}
	return false, nil
}

func (s *stubSubscriberRepo) Delete(ctx context.Context, name string) error {
	filtered := s.list[:0]
	for _, subscriber := range s.list {
		if subscriber.Name() != name {
			filtered = append(filtered, subscriber)
		}
	}
	s.list = filtered
	return nil
}

func TestProcedureGenerator_GenerateSubscriberProcedure_DeploysPackageUpdate(t *testing.T) {
	domain.DefaultFunnyNameGenerator().Reset()

	stub := &stubDBRepo{
		procedureExists: map[string]bool{buildProcedureName("BARNACLE"): false},
	}
	pg := NewProcedureGenerator(stub)
	subscriber, err := domain.NewSubscriberWithFunnyName("TEST_SUB", "BARNACLE", domain.DefaultBatchSize, domain.DefaultWaitTime)
	if err != nil {
		t.Fatalf("NewSubscriberWithFunnyName() returned error: %v", err)
	}

	if err := pg.GenerateSubscriberProcedure(context.Background(), subscriber); err != nil {
		t.Fatalf("GenerateSubscriberProcedure() returned error: %v", err)
	}

	if stub.deployFileCallCount != 1 {
		t.Fatalf("expected 1 package deploy, got %d", stub.deployFileCallCount)
	}
	if !strings.Contains(stub.deployedSQL, "PROCEDURE TRACE_MESSAGE_BARNACLE(") {
		t.Fatalf("generated deployment SQL missing procedure declaration: %s", stub.deployedSQL)
	}
	if !strings.Contains(stub.deployedSQL, "subscriber_name_ => 'BARNACLE'") {
		t.Fatalf("generated deployment SQL missing subscriber alias: %s", stub.deployedSQL)
	}
	if !strings.Contains(stub.deployedSQL, "PROCEDURE Enqueue_For_Subscriber(") {
		t.Fatalf("generated deployment SQL missing base helper procedure: %s", stub.deployedSQL)
	}
}

func TestProcedureGenerator_GenerateSubscriberProcedure_SkipsExistingProcedure(t *testing.T) {
	domain.DefaultFunnyNameGenerator().Reset()

	stub := &stubDBRepo{
		procedureExists: map[string]bool{buildProcedureName("BARNACLE"): true},
	}
	pg := NewProcedureGenerator(stub)
	subscriber, err := domain.NewSubscriberWithFunnyName("TEST_SUB", "BARNACLE", domain.DefaultBatchSize, domain.DefaultWaitTime)
	if err != nil {
		t.Fatalf("NewSubscriberWithFunnyName() returned error: %v", err)
	}

	if err := pg.GenerateSubscriberProcedure(context.Background(), subscriber); err != nil {
		t.Fatalf("GenerateSubscriberProcedure() returned error: %v", err)
	}

	if stub.deployFileCallCount != 0 {
		t.Fatalf("expected no package deploy when procedure exists, got %d", stub.deployFileCallCount)
	}
}

func TestProcedureGenerator_DropSubscriberProcedure_RedeploysPackageWithoutProcedure(t *testing.T) {
	stub := &stubDBRepo{
		procedureExists: map[string]bool{buildProcedureName("BARNACLE"): true},
		packageSpecSource: splitLines(`CREATE OR REPLACE PACKAGE OMNI_TRACER_API AS
    PROCEDURE Enqueue_For_Subscriber(
        subscriber_name_ IN VARCHAR2,
        message_         IN CLOB,
        log_level_       IN VARCHAR2 DEFAULT 'INFO'
    );

    PROCEDURE TRACE_MESSAGE_BARNACLE(
        message_   IN CLOB,
        log_level_ IN VARCHAR2 DEFAULT 'INFO'
    );
END OMNI_TRACER_API;`),
		packageBodySource: splitLines(`CREATE OR REPLACE PACKAGE BODY OMNI_TRACER_API AS
    PROCEDURE Enqueue_For_Subscriber(
        subscriber_name_ IN VARCHAR2,
        message_         IN CLOB,
        log_level_       IN VARCHAR2 DEFAULT 'INFO'
    )
    IS
    BEGIN
        NULL;
    END Enqueue_For_Subscriber;

    PROCEDURE TRACE_MESSAGE_BARNACLE(
        message_   IN CLOB,
        log_level_ IN VARCHAR2 DEFAULT 'INFO'
    )
    IS
    BEGIN
        NULL;
    END TRACE_MESSAGE_BARNACLE;
END OMNI_TRACER_API;`),
	}
	pg := NewProcedureGenerator(stub)

	if err := pg.DropSubscriberProcedure(context.Background(), "BARNACLE"); err != nil {
		t.Fatalf("DropSubscriberProcedure() returned error: %v", err)
	}

	if stub.deployFileCallCount != 1 {
		t.Fatalf("expected 1 package deploy, got %d", stub.deployFileCallCount)
	}
	if strings.Contains(stub.deployedSQL, "TRACE_MESSAGE_BARNACLE") {
		t.Fatalf("package deployment still contains dropped procedure: %s", stub.deployedSQL)
	}
}

func TestSubscriberService_RegisterSubscriber_DoesNotPersistBeforeProcedureGenerationSucceeds(t *testing.T) {
	domain.DefaultFunnyNameGenerator().Reset()

	db := &stubDBRepo{deployFileErr: errors.New("deploy failed")}
	repo := &stubSubscriberRepo{}
	service := NewSubscriberService(db, repo, NewProcedureGenerator(db))

	_, err := service.RegisterSubscriber(context.Background())
	if err == nil {
		t.Fatal("expected RegisterSubscriber() to fail when package deployment fails")
	}
	if len(repo.saved) != 0 {
		t.Fatalf("expected no persisted subscriber on failure, got %d saves", len(repo.saved))
	}
}

func TestSubscriberService_RegisterSubscriber_PersistsFunnyNameAndUsesConsumerAlias(t *testing.T) {
	domain.DefaultFunnyNameGenerator().Reset()

	db := &stubDBRepo{}
	repo := &stubSubscriberRepo{}
	service := NewSubscriberService(db, repo, NewProcedureGenerator(db))

	subscriber, err := service.RegisterSubscriber(context.Background())
	if err != nil {
		t.Fatalf("RegisterSubscriber() returned error: %v", err)
	}

	if subscriber.FunnyName() == "" {
		t.Fatal("expected subscriber funny name to be assigned")
	}
	if len(repo.saved) != 1 {
		t.Fatalf("expected 1 persisted subscriber, got %d", len(repo.saved))
	}
	if repo.saved[0].FunnyName() != subscriber.FunnyName() {
		t.Fatalf("saved funny name mismatch: got %q want %q", repo.saved[0].FunnyName(), subscriber.FunnyName())
	}
	if len(db.registeredConsumers) != 1 {
		t.Fatalf("expected 1 Oracle subscriber registration, got %d", len(db.registeredConsumers))
	}
	if db.registeredConsumers[0] != subscriber.FunnyName() {
		t.Fatalf("expected Oracle registration to use funny name %q, got %q", subscriber.FunnyName(), db.registeredConsumers[0])
	}
	if subscriber.ConsumerName() != subscriber.FunnyName() {
		t.Fatalf("expected consumer name %q, got %q", subscriber.FunnyName(), subscriber.ConsumerName())
	}
	if !strings.Contains(db.deployedSQL, "subscriber_name_ => '"+subscriber.FunnyName()+"'") {
		t.Fatalf("generated deployment SQL did not target subscriber alias %q", subscriber.FunnyName())
	}
}

func TestValidateFunnyNameForProcedure(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{name: "Mickey", wantErr: false},
		{name: "BARNACLE", wantErr: false},
		{name: "Pickles", wantErr: false},
		{name: "", wantErr: true},
		{name: "Invalid123", wantErr: true},
	}

	for _, tc := range tests {
		err := validateFunnyNameForProcedure(tc.name)
		if (err != nil) != tc.wantErr {
			t.Errorf("validateFunnyNameForProcedure(%q) error = %v, wantErr %v", tc.name, err, tc.wantErr)
		}
	}
}

func splitLines(input string) []string {
	return strings.Split(input, "\n")
}
