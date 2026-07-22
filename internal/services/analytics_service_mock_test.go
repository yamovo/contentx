package services

import (
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/repository"
	"gorm.io/gorm"
)

func TestMockAnalytics_NewWithRepo(t *testing.T) {
	svc := NewAnalyticsServiceWithRepo(&MockAnalyticsRepository{})
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

// ---------- Dashboard ----------

func TestMockAnalytics_Dashboard_Success(t *testing.T) {
	repo := &MockAnalyticsRepository{
		DashboardStatsData: repository.DashboardStatsData{
			Articles: 10, Published: 8, Comments: 50, Users: 5, Media: 20,
			ViewsToday: 100, TotalViews: 1000,
		},
		RecentArticlesData: []models.Article{{BaseModel: models.BaseModel{ID: 1}, Title: "latest"}},
		RecentCommentsData: []models.Comment{{BaseModel: models.BaseModel{ID: 1}, Content: "hi"}},
		PopularData:        []models.Article{{BaseModel: models.BaseModel{ID: 2}, Title: "popular"}},
	}
	svc := NewAnalyticsServiceWithRepo(repo)

	data, err := svc.Dashboard()
	if err != nil {
		t.Fatalf("Dashboard failed: %v", err)
	}
	if data.Stats.Articles != 10 {
		t.Errorf("expected 10 articles, got %d", data.Stats.Articles)
	}
	if data.Stats.Published != 8 {
		t.Errorf("expected 8 published, got %d", data.Stats.Published)
	}
	if len(data.RecentArticles) != 1 {
		t.Errorf("expected 1 recent article, got %d", len(data.RecentArticles))
	}
	if len(data.PopularArticles) != 1 {
		t.Errorf("expected 1 popular article, got %d", len(data.PopularArticles))
	}
}

// ---------- ViewsOverTime ----------

func TestMockAnalytics_ViewsOverTime_Success(t *testing.T) {
	// Provide one data point for today; fillDateGaps should expand to `days` entries.
	today := time.Now().Format("2006-01-02")
	repo := &MockAnalyticsRepository{
		ViewsOverTimeData: []repository.DayStatsData{{Date: today, Views: 42}},
	}
	svc := NewAnalyticsServiceWithRepo(repo)

	result, err := svc.ViewsOverTime(7)
	if err != nil {
		t.Fatalf("ViewsOverTime failed: %v", err)
	}
	if len(result) != 7 {
		t.Fatalf("expected 7 day stats (filled), got %d", len(result))
	}
	// The last entry should be today with 42 views.
	last := result[len(result)-1]
	if last.Date != today {
		t.Errorf("expected last date %s, got %s", today, last.Date)
	}
	if last.Views != 42 {
		t.Errorf("expected last views 42, got %d", last.Views)
	}
	// Other days should have 0 views.
	if result[0].Views != 0 {
		t.Errorf("expected first day views 0, got %d", result[0].Views)
	}
}

func TestMockAnalytics_ViewsOverTime_DaysLessThan1(t *testing.T) {
	repo := &MockAnalyticsRepository{
		ViewsOverTimeData: []repository.DayStatsData{},
	}
	svc := NewAnalyticsServiceWithRepo(repo)

	// days < 1 → defaults to 30. Empty data → fillDateGaps returns empty.
	result, err := svc.ViewsOverTime(0)
	if err != nil {
		t.Fatalf("ViewsOverTime failed: %v", err)
	}
	// Empty input → fillDateGaps returns empty (per implementation).
	if len(result) != 0 {
		t.Errorf("expected 0 results for empty data, got %d", len(result))
	}
}

func TestMockAnalytics_ViewsOverTime_Error(t *testing.T) {
	repo := &MockAnalyticsRepository{ViewsOverTimeErr: gorm.ErrInvalidDB}
	svc := NewAnalyticsServiceWithRepo(repo)

	_, err := svc.ViewsOverTime(7)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------- TopReferrers ----------

func TestMockAnalytics_TopReferrers_Success(t *testing.T) {
	repo := &MockAnalyticsRepository{
		TopReferrersData: []repository.ReferrerData{
			{Referrer: "https://google.com", Count: 100},
			{Referrer: "https://twitter.com", Count: 50},
		},
	}
	svc := NewAnalyticsServiceWithRepo(repo)

	result, err := svc.TopReferrers()
	if err != nil {
		t.Fatalf("TopReferrers failed: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 referrers, got %d", len(result))
	}
	if result[0].Referrer != "https://google.com" || result[0].Count != 100 {
		t.Errorf("unexpected first referrer: %+v", result[0])
	}
}

func TestMockAnalytics_TopReferrers_Error(t *testing.T) {
	repo := &MockAnalyticsRepository{TopReferrersErr: gorm.ErrInvalidDB}
	svc := NewAnalyticsServiceWithRepo(repo)

	_, err := svc.TopReferrers()
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------- DeviceBreakdown ----------

func TestMockAnalytics_DeviceBreakdown_Success(t *testing.T) {
	repo := &MockAnalyticsRepository{
		DeviceBreakdownData: repository.DeviceBreakdownData{
			Devices:  []repository.BreakdownData{{Name: "desktop", Count: 80}, {Name: "mobile", Count: 20}},
			Browsers: []repository.BreakdownData{{Name: "Chrome", Count: 60}},
			OS:       []repository.BreakdownData{{Name: "Windows", Count: 50}},
		},
	}
	svc := NewAnalyticsServiceWithRepo(repo)

	result, err := svc.DeviceBreakdown()
	if err != nil {
		t.Fatalf("DeviceBreakdown failed: %v", err)
	}
	if len(result.Devices) != 2 {
		t.Errorf("expected 2 devices, got %d", len(result.Devices))
	}
	if result.Devices[0].Name != "desktop" || result.Devices[0].Count != 80 {
		t.Errorf("unexpected first device: %+v", result.Devices[0])
	}
	if len(result.Browsers) != 1 || result.Browsers[0].Name != "Chrome" {
		t.Errorf("unexpected browsers: %+v", result.Browsers)
	}
	if len(result.OS) != 1 || result.OS[0].Name != "Windows" {
		t.Errorf("unexpected OS: %+v", result.OS)
	}
}

func TestMockAnalytics_DeviceBreakdown_Error(t *testing.T) {
	repo := &MockAnalyticsRepository{DeviceBreakdownErr: gorm.ErrInvalidDB}
	svc := NewAnalyticsServiceWithRepo(repo)

	_, err := svc.DeviceBreakdown()
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------- RecordView ----------

func TestMockAnalytics_RecordView_Success(t *testing.T) {
	repo := &MockAnalyticsRepository{}
	svc := NewAnalyticsServiceWithRepo(repo)

	aid := uint(1)
	err := svc.RecordView(RecordViewRequest{
		ArticleID: &aid, Path: "/articles/hello", Duration: 30,
	}, "1.2.3.4", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0", "https://google.com", "session-123")
	if err != nil {
		t.Fatalf("RecordView failed: %v", err)
	}
	if len(repo.CreatedPageViews) != 1 {
		t.Fatalf("expected 1 page view, got %d", len(repo.CreatedPageViews))
	}
	pv := repo.CreatedPageViews[0]
	if pv.Path != "/articles/hello" {
		t.Errorf("expected path /articles/hello, got %s", pv.Path)
	}
	if pv.Device != "desktop" {
		t.Errorf("expected device desktop, got %s", pv.Device)
	}
	if pv.Browser != "Chrome" {
		t.Errorf("expected browser Chrome, got %s", pv.Browser)
	}
	if pv.OS != "Windows" {
		t.Errorf("expected OS Windows, got %s", pv.OS)
	}
	if pv.Referrer != "https://google.com" {
		t.Errorf("expected referrer https://google.com, got %s", pv.Referrer)
	}
}

func TestMockAnalytics_RecordView_MobileUA(t *testing.T) {
	repo := &MockAnalyticsRepository{}
	svc := NewAnalyticsServiceWithRepo(repo)

	err := svc.RecordView(RecordViewRequest{
		Path: "/test",
	}, "1.2.3.4", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) Safari/604.1", "", "s1")
	if err != nil {
		t.Fatalf("RecordView failed: %v", err)
	}
	pv := repo.CreatedPageViews[0]
	if pv.Device != "mobile" {
		t.Errorf("expected device mobile, got %s", pv.Device)
	}
	if pv.Browser != "Safari" {
		t.Errorf("expected browser Safari, got %s", pv.Browser)
	}
}

func TestMockAnalytics_RecordView_Error(t *testing.T) {
	repo := &MockAnalyticsRepository{CreatePageViewErr: gorm.ErrInvalidDB}
	svc := NewAnalyticsServiceWithRepo(repo)

	err := svc.RecordView(RecordViewRequest{Path: "/test"}, "1.2.3.4", "ua", "", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------- Pure helper functions ----------

func TestFillDateGaps_Empty(t *testing.T) {
	result := fillDateGaps(nil, 7)
	if len(result) != 0 {
		t.Errorf("expected 0 results for empty input, got %d", len(result))
	}
}

func TestFillDateGaps_WithGaps(t *testing.T) {
	// Provide a data point for 3 days ago.
	threeDaysAgo := time.Now().AddDate(0, 0, -2).Format("2006-01-02")
	data := []DayStats{{Date: threeDaysAgo, Views: 10}}

	result := fillDateGaps(data, 5)
	if len(result) != 5 {
		t.Fatalf("expected 5 filled days, got %d", len(result))
	}
	// Find the day with data.
	found := false
	for _, d := range result {
		if d.Date == threeDaysAgo && d.Views == 10 {
			found = true
		}
	}
	if !found {
		t.Error("expected to find the original data point in filled results")
	}
}

func TestDetectDevice(t *testing.T) {
	cases := []struct {
		ua   string
		want string
	}{
		{"Mozilla/5.0 (iPhone)", "mobile"},
		{"Mozilla/5.0 (Android)", "mobile"},
		{"Mozilla/5.0 (iPad; CPU OS 13_0)", "tablet"},
		{"Mozilla/5.0 (Windows NT 10.0)", "desktop"},
		{"unknown-agent", "desktop"},
	}
	for _, tc := range cases {
		if got := detectDevice(tc.ua); got != tc.want {
			t.Errorf("detectDevice(%q) = %s, want %s", tc.ua, got, tc.want)
		}
	}
}

func TestDetectBrowser(t *testing.T) {
	cases := []struct {
		ua   string
		want string
	}{
		{"Mozilla/5.0 Edg/120.0", "Edge"},
		{"Mozilla/5.0 Chrome/120.0", "Chrome"},
		{"Mozilla/5.0 Firefox/120.0", "Firefox"},
		{"Mozilla/5.0 Safari/604.1", "Safari"},
		{"unknown-agent", "Other"},
	}
	for _, tc := range cases {
		if got := detectBrowser(tc.ua); got != tc.want {
			t.Errorf("detectBrowser(%q) = %s, want %s", tc.ua, got, tc.want)
		}
	}
}

func TestDetectOS(t *testing.T) {
	cases := []struct {
		ua   string
		want string
	}{
		{"Mozilla/5.0 (Windows NT 10.0)", "Windows"},
		{"Mozilla/5.0 (Mac OS X)", "macOS"},
		{"Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X)", "iOS"},
		{"Mozilla/5.0 (iPad; CPU OS 16_0 like Mac OS X)", "iOS"},
		{"Mozilla/5.0 (Linux; Android 13)", "Android"},
		{"Mozilla/5.0 (Linux; X11)", "Linux"},
		{"unknown-agent", "Other"},
	}
	for _, tc := range cases {
		if got := detectOS(tc.ua); got != tc.want {
			t.Errorf("detectOS(%q) = %s, want %s", tc.ua, got, tc.want)
		}
	}
}

func TestStringifyValue(t *testing.T) {
	cases := []struct {
		input interface{}
		want  string
	}{
		{"hello", "hello"},
		{float64(42), "42"},
		{float64(3.14), "3.14"},
		{true, "true"},
		{false, "false"},
		{[]string{"a", "b"}, "[a b]"},
	}
	for _, tc := range cases {
		if got := stringifyValue(tc.input); got != tc.want {
			t.Errorf("stringifyValue(%v) = %s, want %s", tc.input, got, tc.want)
		}
	}
}

func TestDetectType(t *testing.T) {
	cases := []struct {
		input interface{}
		want  string
	}{
		{true, "bool"},
		{float64(42), "int"},
		{"hello", "string"},
		{[]string{}, "string"},
	}
	for _, tc := range cases {
		if got := detectType(tc.input); got != tc.want {
			t.Errorf("detectType(%v) = %s, want %s", tc.input, got, tc.want)
		}
	}
}
