package ai_test

import (
	"testing"

	"webreaper/internal/usecase/port"
)

// 采样矩阵·引擎分片（v2 去缓存核心）：问法池按引擎错位取用——
// 同引擎多次采样不同问法、不同引擎问法起点不同（相同 prompt 不跨引擎命中缓存）。

func TestShardQuestions_Isolation(t *testing.T) {
	pool := []string{"q0", "q1", "q2", "q3", "q4", "q5", "q6", "q7"}
	e0 := port.ShardQuestions(pool, 0, 3)
	e1 := port.ShardQuestions(pool, 1, 3)
	e2 := port.ShardQuestions(pool, 2, 3)
	t.Logf("e0=%v e1=%v e2=%v", e0, e1, e2)
	// 同引擎内不重复
	seen := map[string]bool{}
	for _, q := range e0 {
		if seen[q] {
			t.Errorf("引擎 0 内重复问法: %s", q)
		}
		seen[q] = true
	}
	// 引擎间起点错位（不同首问法）
	if e0[0] == e1[0] || e1[0] == e2[0] {
		t.Errorf("引擎间首问法不应相同: %v vs %v vs %v", e0, e1, e2)
	}
}

func TestShardQuestions_SmallPool(t *testing.T) {
	// 池只有 4 个，2 引擎各 3 采样——环形错位不越界
	pool := []string{"q0", "q1", "q2", "q3"}
	e0 := port.ShardQuestions(pool, 0, 3)
	e1 := port.ShardQuestions(pool, 1, 3)
	if len(e0) != 3 || len(e1) != 3 {
		t.Fatalf("分片长度错误: %d %d", len(e0), len(e1))
	}
	if e0[0] == e1[0] {
		t.Errorf("小池也需错位: %v vs %v", e0, e1)
	}
}

func TestShardQuestions_Empty(t *testing.T) {
	if port.ShardQuestions(nil, 0, 3) != nil {
		t.Error("空池应返回 nil（probe 模板兜底）")
	}
}
