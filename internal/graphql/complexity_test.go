package graphql

import "testing"

// ─── SEC-11：query complexity 评分 ──────────────────────────────────────────

func TestQueryComplexity(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  int
	}{
		// 单字段：1
		{"single field", `{ feed }`, 1},
		// 两个顶层标量：2
		{"two scalars", `{ feed sitemap }`, 2},
		// 嵌套无分页参数：articles(1) + 1*items(1) + 1*title(1) = 3
		{"plain nesting", `{ articles { items { title } } }`, 3},
		// 分页参数字面量：articles(1) + 50*(items(1)+1*title(1)) = 101
		{"pageSize literal", `{ articles(pageSize: 50) { items { title } } }`, 101},
		// 分页参数超上限被钳制为 100：1 + 100*2 = 201
		{"pageSize clamped", `{ articles(pageSize: 5000) { items { title } } }`, 201},
		// 变量分页参数取默认乘数 20：1 + 20*2 = 41
		{"pageSize variable", `query Q($n: Int!) { articles(pageSize: $n) { items { title } } }`, 41},
		// fragment spread 参与评分：articles(1) + 1*(items(1) + 1*(tags(1)+1*name(1))) = 4
		{"fragment spread", `
			fragment F on Article { tags { name } }
			{ articles { items { ...F } } }
		`, 4},
		// 解析失败：0（交给 graphql.Do 报错）
		{"invalid query", `not valid graphql!!!`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := queryComplexity(tc.query)
			if got != tc.want {
				t.Errorf("queryComplexity() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestQueryComplexity_WideQueryExceedsMax(t *testing.T) {
	// 宽查询：多个大 pageSize 的兄弟列表字段，深度合法但代价必须超限。
	q := `{
		a1: articles(pageSize: 100) { items { title content excerpt author { username } } }
		a2: articles(pageSize: 100) { items { title content excerpt author { username } } }
		a3: articles(pageSize: 100) { items { title content excerpt author { username } } }
		a4: articles(pageSize: 100) { items { title content excerpt author { username } } }
	}`
	if depth := queryDepth(q); depth > maxQueryDepth {
		t.Fatalf("test premise broken: depth %d should be legal (≤ %d)", depth, maxQueryDepth)
	}
	if cost := queryComplexity(q); cost <= maxQueryComplexity {
		t.Errorf("SEC-11: wide query cost %d should exceed max %d", cost, maxQueryComplexity)
	}
}

func TestQueryComplexity_NormalQueryPasses(t *testing.T) {
	// 正常业务查询必须远低于上限，避免误杀。
	q := `{
		articles(page: 1, pageSize: 20) {
			items { id title slug excerpt author { username } category { name } tags { name } }
			total totalPages
		}
	}`
	cost := queryComplexity(q)
	if cost > maxQueryComplexity {
		t.Errorf("normal query cost %d must not exceed max %d", cost, maxQueryComplexity)
	}
	if cost == 0 {
		t.Error("normal query should have non-zero cost")
	}
}
