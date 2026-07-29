package services

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/cache"
	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/repository"
)

func TestArticleCache_ListHitAndInvalidate(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")
	svc.WithCache(cache.NewMemoryDriver(1000), 5*time.Minute)
	user := createTestUser(t, db, "cache-author", "author")
	createTestArticle(t, db, user.ID, "Cached Article")

	// First list: DB hit, result cached.
	r1, err := svc.List(ListArticlesFilter{Status: "published", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if r1.Total != 1 {
		t.Fatalf("total = %d, want 1", r1.Total)
	}

	// Create a new published article (invalidates list cache).
	_, err = svc.Create(CreateArticleRequest{Title: "New One", Status: "published"}, user.ID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// List again: should see 2 articles (cache was invalidated by Create).
	r2, err := svc.List(ListArticlesFilter{Status: "published", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("List after create: %v", err)
	}
	if r2.Total != 2 {
		t.Fatalf("after create, total = %d, want 2", r2.Total)
	}
}

func TestArticleCache_GetHitAndInvalidate(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")
	svc.WithCache(cache.NewMemoryDriver(1000), 5*time.Minute)
	user := createTestUser(t, db, "cache-author2", "author")
	article := createTestArticle(t, db, user.ID, "To Cache")

	// First Get: from DB, result cached.
	a1, err := svc.Get(article.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if a1.Title != "To Cache" {
		t.Fatalf("title = %q, want To Cache", a1.Title)
	}

	// Update the title (invalidates detail cache for this ID).
	newTitle := "Updated Title"
	_, err = svc.Update(article.ID, UpdateArticleRequest{Title: &newTitle}, user.ID, true)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	// Get again: should see updated title (cache invalidated, re-fetches from DB).
	a2, err := svc.Get(article.ID)
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if a2.Title != "Updated Title" {
		t.Fatalf("after update, title = %q, want Updated Title", a2.Title)
	}
}

// ─── SEC-9：single-flight 防击穿 ─────────────────────────────────────────────

// countingArticleRepo 包装 MockArticleRepository，统计回源次数并模拟慢查询，
// 使并发 miss 窗口重叠，验证 single-flight 合并效果。
type countingArticleRepo struct {
	*MockArticleRepository
	getCalls  int64
	listCalls int64
}

func (r *countingArticleRepo) GetByID(id uint) (*models.Article, error) {
	atomic.AddInt64(&r.getCalls, 1)
	time.Sleep(50 * time.Millisecond)
	return r.MockArticleRepository.GetByID(id)
}

func (r *countingArticleRepo) List(f repository.ArticleListFilter) ([]models.Article, int64, error) {
	atomic.AddInt64(&r.listCalls, 1)
	time.Sleep(50 * time.Millisecond)
	return r.MockArticleRepository.List(f)
}

func TestArticleCache_SingleFlightGet(t *testing.T) {
	repo := &countingArticleRepo{MockArticleRepository: &MockArticleRepository{
		Articles: map[uint]*models.Article{1: {BaseModel: models.BaseModel{ID: 1}, Title: "SF"}},
	}}
	svc := NewArticleServiceWithRepo(repo, "http://localhost:8080")
	svc.WithCache(cache.NewMemoryDriver(1000), 5*time.Minute)

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			if _, err := svc.Get(1); err != nil {
				t.Errorf("Get: %v", err)
			}
		}()
	}
	wg.Wait()

	if n := atomic.LoadInt64(&repo.getCalls); n != 1 {
		t.Errorf("SEC-9: %d concurrent misses hit repo %d times, want 1 (single-flight)", goroutines, n)
	}
}

func TestArticleCache_SingleFlightList(t *testing.T) {
	repo := &countingArticleRepo{MockArticleRepository: &MockArticleRepository{
		ArticlesList: []models.Article{{BaseModel: models.BaseModel{ID: 1}, Title: "SF"}},
		ListTotal:    1,
	}}
	svc := NewArticleServiceWithRepo(repo, "http://localhost:8080")
	svc.WithCache(cache.NewMemoryDriver(1000), 5*time.Minute)

	filter := ListArticlesFilter{Status: "published", Page: 1, PageSize: 20}
	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			if _, err := svc.List(filter); err != nil {
				t.Errorf("List: %v", err)
			}
		}()
	}
	wg.Wait()

	if n := atomic.LoadInt64(&repo.listCalls); n != 1 {
		t.Errorf("SEC-9: %d concurrent misses hit repo %d times, want 1 (single-flight)", goroutines, n)
	}
}
