package services

import (
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// ──────────────────────────────────────────────────────────────────────────────
// 状态机规则测试（models.AllowedTransition）
// ──────────────────────────────────────────────────────────────────────────────

func TestAllowedTransition_LegalPaths(t *testing.T) {
	cases := []struct {
		from, to models.ArticleStatus
	}{
		{models.StatusDraft, models.StatusPending},
		{models.StatusDraft, models.StatusPublished},
		{models.StatusDraft, models.StatusScheduled},
		{models.StatusDraft, models.StatusArchived},
		{models.StatusDraft, models.StatusTrash},
		{models.StatusPending, models.StatusDraft},
		{models.StatusPending, models.StatusPublished},
		{models.StatusPending, models.StatusArchived},
		{models.StatusPending, models.StatusTrash},
		{models.StatusPublished, models.StatusDraft},
		{models.StatusPublished, models.StatusArchived},
		{models.StatusPublished, models.StatusTrash},
		{models.StatusScheduled, models.StatusDraft},
		{models.StatusScheduled, models.StatusPublished},
		{models.StatusScheduled, models.StatusArchived},
		{models.StatusScheduled, models.StatusTrash},
		{models.StatusArchived, models.StatusDraft},
		{models.StatusArchived, models.StatusTrash},
		{models.StatusTrash, models.StatusDraft},
		{models.StatusTrash, models.StatusPublished},
	}
	for _, c := range cases {
		if !models.AllowedTransition(c.from, c.to) {
			t.Errorf("expected transition %s → %s to be allowed", c.from, c.to)
		}
	}
}

func TestAllowedTransition_IllegalPaths(t *testing.T) {
	cases := []struct {
		from, to models.ArticleStatus
	}{
		// pending 不能直接到 scheduled（必须先回 draft）
		{models.StatusPending, models.StatusScheduled},
		// published 不能直接到 scheduled / pending
		{models.StatusPublished, models.StatusScheduled},
		{models.StatusPublished, models.StatusPending},
		// archived 不能到 published / pending / scheduled
		{models.StatusArchived, models.StatusPublished},
		{models.StatusArchived, models.StatusPending},
		{models.StatusArchived, models.StatusScheduled},
		// trash 不能到 scheduled / pending / archived
		{models.StatusTrash, models.StatusScheduled},
		{models.StatusTrash, models.StatusPending},
		{models.StatusTrash, models.StatusArchived},
	}
	for _, c := range cases {
		if models.AllowedTransition(c.from, c.to) {
			t.Errorf("expected transition %s → %s to be forbidden", c.from, c.to)
		}
	}
}

func TestAllowedTransition_NoOpAlwaysAllowed(t *testing.T) {
	statuses := []models.ArticleStatus{
		models.StatusDraft, models.StatusPublished, models.StatusPending,
		models.StatusScheduled, models.StatusArchived, models.StatusTrash,
	}
	for _, s := range statuses {
		if !models.AllowedTransition(s, s) {
			t.Errorf("expected no-op transition %s → %s to be allowed", s, s)
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// ArticleService 工作流方法测试
// ──────────────────────────────────────────────────────────────────────────────

// newWorkflowService 构造一个带 mock repo 的 ArticleService，预置一篇指定
// 状态的文章。返回 service 和 repo 以便断言。
func newWorkflowService(initialStatus models.ArticleStatus) (*ArticleService, *MockArticleRepository) {
	repo := &MockArticleRepository{
		Articles: map[uint]*models.Article{
			1: {BaseModel: models.BaseModel{ID: 1}, Title: "T", Status: initialStatus},
		},
	}
	s := NewArticleServiceWithRepo(repo, "http://localhost:8080")
	return s, repo
}

func TestWorkflow_Publish_FromDraft(t *testing.T) {
	s, repo := newWorkflowService(models.StatusDraft)
	wh := &MockWebhookDispatcher{}
	s.SetWebhookDispatcher(wh)

	article, err := s.Publish(1)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
	if article.Status != models.StatusPublished {
		t.Errorf("Status = %s, want published", article.Status)
	}
	if len(repo.UpdateStatusCalls) != 1 {
		t.Fatalf("expected 1 UpdateStatus call, got %d", len(repo.UpdateStatusCalls))
	}
	if repo.UpdateStatusCalls[0].Status != string(models.StatusPublished) {
		t.Errorf("UpdateStatus status = %q, want published", repo.UpdateStatusCalls[0].Status)
	}
	if repo.UpdateStatusCalls[0].PublishedAt == nil {
		t.Error("expected PublishedAt to be set for first publish")
	}
	// webhook entry.publish 应触发
	if last, ok := wh.Last(); !ok || last.Event != models.WebhookEventEntryPublish {
		t.Errorf("expected entry.publish webhook, got %+v", last)
	}
}

func TestWorkflow_Publish_AlreadyPublished_KeepsPublishedAt(t *testing.T) {
	// 已发布文章再次 publish 不应重置 PublishedAt
	existing := time.Now().Add(-1 * time.Hour)
	repo := &MockArticleRepository{
		Articles: map[uint]*models.Article{
			1: {BaseModel: models.BaseModel{ID: 1}, Status: models.StatusPublished, PublishedAt: &existing},
		},
	}
	s := NewArticleServiceWithRepo(repo, "")
	_, err := s.Publish(1)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
	if len(repo.UpdateStatusCalls) != 1 {
		t.Fatalf("expected 1 UpdateStatus call, got %d", len(repo.UpdateStatusCalls))
	}
	if repo.UpdateStatusCalls[0].PublishedAt != nil {
		t.Error("expected PublishedAt to be nil (already set), but it was overwritten")
	}
}

func TestWorkflow_Publish_IllegalFromArchived(t *testing.T) {
	s, _ := newWorkflowService(models.StatusArchived)
	_, err := s.Publish(1)
	if err == nil {
		t.Fatal("expected error for archived → published (illegal)")
	}
}

func TestWorkflow_Publish_NotFound(t *testing.T) {
	repo := &MockArticleRepository{
		Articles: map[uint]*models.Article{},
	}
	s := NewArticleServiceWithRepo(repo, "")
	_, err := s.Publish(999)
	if err == nil {
		t.Fatal("expected error for non-existent article")
	}
}

func TestWorkflow_Unpublish_FromPublished(t *testing.T) {
	s, repo := newWorkflowService(models.StatusPublished)
	wh := &MockWebhookDispatcher{}
	s.SetWebhookDispatcher(wh)

	article, err := s.Unpublish(1)
	if err != nil {
		t.Fatalf("Unpublish failed: %v", err)
	}
	if article.Status != models.StatusDraft {
		t.Errorf("Status = %s, want draft", article.Status)
	}
	if len(repo.UpdateStatusCalls) != 1 {
		t.Fatalf("expected 1 UpdateStatus call, got %d", len(repo.UpdateStatusCalls))
	}
	if repo.UpdateStatusCalls[0].Status != string(models.StatusDraft) {
		t.Errorf("UpdateStatus status = %q, want draft", repo.UpdateStatusCalls[0].Status)
	}
	if last, ok := wh.Last(); !ok || last.Event != models.WebhookEventEntryUnpublish {
		t.Errorf("expected entry.unpublish webhook, got %+v", last)
	}
}

func TestWorkflow_Unpublish_FromArchived_RestoresToDraft(t *testing.T) {
	// archived → draft 是状态机中合法的"恢复"路径（与 trash → draft 同类），
	// 因此 Unpublish 从 archived 应当成功，而不是报错。
	s, repo := newWorkflowService(models.StatusArchived)
	article, err := s.Unpublish(1)
	if err != nil {
		t.Fatalf("Unpublish from archived should succeed (restore path): %v", err)
	}
	if article.Status != models.StatusDraft {
		t.Errorf("Status = %s, want draft", article.Status)
	}
	if len(repo.UpdateStatusCalls) != 1 {
		t.Fatalf("expected 1 UpdateStatus call, got %d", len(repo.UpdateStatusCalls))
	}
	if repo.UpdateStatusCalls[0].Status != string(models.StatusDraft) {
		t.Errorf("UpdateStatus status = %q, want draft", repo.UpdateStatusCalls[0].Status)
	}
}

func TestWorkflow_SubmitForReview_FromDraft(t *testing.T) {
	s, repo := newWorkflowService(models.StatusDraft)
	article, err := s.SubmitForReview(1)
	if err != nil {
		t.Fatalf("SubmitForReview failed: %v", err)
	}
	if article.Status != models.StatusPending {
		t.Errorf("Status = %s, want pending", article.Status)
	}
	if len(repo.UpdateStatusCalls) != 1 {
		t.Fatalf("expected 1 UpdateStatus call, got %d", len(repo.UpdateStatusCalls))
	}
}

func TestWorkflow_SubmitForReview_IllegalFromPublished(t *testing.T) {
	s, _ := newWorkflowService(models.StatusPublished)
	_, err := s.SubmitForReview(1)
	if err == nil {
		t.Fatal("expected error for published → pending (illegal)")
	}
}

func TestWorkflow_Approve_FromPending(t *testing.T) {
	s, repo := newWorkflowService(models.StatusPending)
	wh := &MockWebhookDispatcher{}
	s.SetWebhookDispatcher(wh)

	article, err := s.Approve(1)
	if err != nil {
		t.Fatalf("Approve failed: %v", err)
	}
	if article.Status != models.StatusPublished {
		t.Errorf("Status = %s, want published", article.Status)
	}
	if repo.UpdateStatusCalls[0].Status != string(models.StatusPublished) {
		t.Errorf("UpdateStatus status = %q, want published", repo.UpdateStatusCalls[0].Status)
	}
	if last, ok := wh.Last(); !ok || last.Event != models.WebhookEventEntryPublish {
		t.Errorf("expected entry.publish webhook, got %+v", last)
	}
}

func TestWorkflow_Approve_IllegalFromArchived(t *testing.T) {
	s, _ := newWorkflowService(models.StatusArchived)
	_, err := s.Approve(1)
	if err == nil {
		t.Fatal("expected error for archived → published (illegal)")
	}
}

func TestWorkflow_Schedule_FromDraft(t *testing.T) {
	s, repo := newWorkflowService(models.StatusDraft)
	wh := &MockWebhookDispatcher{}
	s.SetWebhookDispatcher(wh)

	at := time.Now().Add(2 * time.Hour)
	article, err := s.Schedule(1, at)
	if err != nil {
		t.Fatalf("Schedule failed: %v", err)
	}
	if article.Status != models.StatusScheduled {
		t.Errorf("Status = %s, want scheduled", article.Status)
	}
	if len(repo.UpdateStatusCalls) != 1 {
		t.Fatalf("expected 1 UpdateStatus call, got %d", len(repo.UpdateStatusCalls))
	}
	if repo.UpdateStatusCalls[0].ScheduledAt == nil {
		t.Error("expected ScheduledAt to be set")
	}
	if !repo.UpdateStatusCalls[0].ScheduledAt.Equal(at) {
		t.Errorf("ScheduledAt = %v, want %v", *repo.UpdateStatusCalls[0].ScheduledAt, at)
	}
	if last, ok := wh.Last(); !ok || last.Event != models.WebhookEventEntrySchedule {
		t.Errorf("expected entry.schedule webhook, got %+v", last)
	}
}

func TestWorkflow_Schedule_ZeroTime(t *testing.T) {
	s, _ := newWorkflowService(models.StatusDraft)
	_, err := s.Schedule(1, time.Time{})
	if err == nil {
		t.Fatal("expected error for zero scheduled_at")
	}
}

func TestWorkflow_Schedule_IllegalFromPending(t *testing.T) {
	s, _ := newWorkflowService(models.StatusPending)
	at := time.Now().Add(1 * time.Hour)
	_, err := s.Schedule(1, at)
	if err == nil {
		t.Fatal("expected error for pending → scheduled (illegal)")
	}
}

func TestWorkflow_Archive_FromPublished(t *testing.T) {
	s, repo := newWorkflowService(models.StatusPublished)
	article, err := s.Archive(1)
	if err != nil {
		t.Fatalf("Archive failed: %v", err)
	}
	if article.Status != models.StatusArchived {
		t.Errorf("Status = %s, want archived", article.Status)
	}
	if len(repo.UpdateStatusCalls) != 1 {
		t.Fatalf("expected 1 UpdateStatus call, got %d", len(repo.UpdateStatusCalls))
	}
}

func TestWorkflow_Archive_IllegalFromTrash(t *testing.T) {
	s, _ := newWorkflowService(models.StatusTrash)
	_, err := s.Archive(1)
	if err == nil {
		t.Fatal("expected error for trash → archived (illegal)")
	}
}

func TestWorkflow_UpdateStatus_RepoError(t *testing.T) {
	repo := &MockArticleRepository{
		Articles: map[uint]*models.Article{
			1: {BaseModel: models.BaseModel{ID: 1}, Status: models.StatusDraft},
		},
		UpdateStatusErr: gorm.ErrInvalidDB,
	}
	s := NewArticleServiceWithRepo(repo, "")
	_, err := s.Publish(1)
	if err == nil {
		t.Fatal("expected error from UpdateStatus")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// PublishDueScheduled 测试（定时发布扫描）
// ──────────────────────────────────────────────────────────────────────────────

func TestWorkflow_PublishDueScheduled_NoDue(t *testing.T) {
	repo := &MockArticleRepository{
		ScheduledDue: []models.Article{}, // 无到期文章
	}
	s := NewArticleServiceWithRepo(repo, "")
	n, err := s.PublishDueScheduled(time.Now())
	if err != nil {
		t.Fatalf("PublishDueScheduled failed: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 published, got %d", n)
	}
	if len(repo.BulkPublishCalls) != 0 {
		t.Errorf("expected 0 BulkPublish calls, got %d", len(repo.BulkPublishCalls))
	}
}

func TestWorkflow_PublishDueScheduled_WithDueArticles(t *testing.T) {
	repo := &MockArticleRepository{
		ScheduledDue: []models.Article{
			{BaseModel: models.BaseModel{ID: 1}, Status: models.StatusScheduled},
			{BaseModel: models.BaseModel{ID: 2}, Status: models.StatusScheduled},
		},
	}
	wh := &MockWebhookDispatcher{}
	s := NewArticleServiceWithRepo(repo, "")
	s.SetWebhookDispatcher(wh)

	n, err := s.PublishDueScheduled(time.Now())
	if err != nil {
		t.Fatalf("PublishDueScheduled failed: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 published, got %d", n)
	}
	if len(repo.BulkPublishCalls) != 1 {
		t.Fatalf("expected 1 BulkPublish call, got %d", len(repo.BulkPublishCalls))
	}
	if len(repo.BulkPublishCalls[0].IDs) != 2 {
		t.Errorf("expected 2 IDs in BulkPublish, got %d", len(repo.BulkPublishCalls[0].IDs))
	}
	// webhook entry.publish 应触发（mode=scheduled）
	if last, ok := wh.Last(); !ok || last.Event != models.WebhookEventEntryPublish {
		t.Errorf("expected entry.publish webhook, got %+v", last)
	}
}

func TestWorkflow_PublishDueScheduled_ListError(t *testing.T) {
	repo := &MockArticleRepository{
		ListScheduledDueErr: gorm.ErrInvalidDB,
	}
	s := NewArticleServiceWithRepo(repo, "")
	_, err := s.PublishDueScheduled(time.Now())
	if err == nil {
		t.Fatal("expected error from ListScheduledDue")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// PublishScheduler 测试
// ──────────────────────────────────────────────────────────────────────────────

// fakeScheduledPublisher 记录 PublishDueScheduled 调用，可控制返回值。
type fakeScheduledPublisher struct {
	calls  int
	result int
	err    error
}

func (f *fakeScheduledPublisher) PublishDueScheduled(now time.Time) (int, error) {
	f.calls++
	return f.result, f.err
}

func TestScheduler_Tick_DelegatesToPublisher(t *testing.T) {
	fake := &fakeScheduledPublisher{result: 3}
	sch := NewPublishScheduler(fake, time.Minute, nil)

	n, err := sch.Tick()
	if err != nil {
		t.Fatalf("Tick failed: %v", err)
	}
	if n != 3 {
		t.Errorf("expected 3, got %d", n)
	}
	if fake.calls != 1 {
		t.Errorf("expected 1 call, got %d", fake.calls)
	}
}

func TestScheduler_Tick_PropagatesError(t *testing.T) {
	fake := &fakeScheduledPublisher{err: gorm.ErrInvalidDB}
	sch := NewPublishScheduler(fake, time.Minute, nil)
	_, err := sch.Tick()
	if err == nil {
		t.Fatal("expected error to propagate")
	}
}

func TestScheduler_StartStop_Lifecycle(t *testing.T) {
	fake := &fakeScheduledPublisher{}
	sch := NewPublishScheduler(fake, 50*time.Millisecond, nil)

	if sch.Running() {
		t.Fatal("expected scheduler to not be running before Start")
	}
	sch.Start()
	if !sch.Running() {
		t.Fatal("expected scheduler to be running after Start")
	}
	// Start is idempotent
	sch.Start()
	if !sch.Running() {
		t.Fatal("expected scheduler to still be running after double Start")
	}
	sch.Stop()
	if sch.Running() {
		t.Fatal("expected scheduler to not be running after Stop")
	}
	// Stop is idempotent
	sch.Stop()
}

func TestScheduler_Start_TriggersTicks(t *testing.T) {
	// 用很短的 interval 验证 ticker 确实在驱动 Tick
	fake := &fakeScheduledPublisher{result: 0}
	sch := NewPublishScheduler(fake, 20*time.Millisecond, nil)
	sch.Start()
	// 等待几轮 tick
	time.Sleep(100 * time.Millisecond)
	sch.Stop()
	if fake.calls < 2 {
		t.Errorf("expected at least 2 ticks in 100ms with 20ms interval, got %d", fake.calls)
	}
}

func TestScheduler_DefaultInterval(t *testing.T) {
	fake := &fakeScheduledPublisher{}
	sch := NewPublishScheduler(fake, 0, nil) // 0 → 默认 1m
	if sch.interval != time.Minute {
		t.Errorf("expected default interval 1m, got %v", sch.interval)
	}
}
