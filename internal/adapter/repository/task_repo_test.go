package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/domain/valueobject"
	"webreaper/internal/pkg"
)

// sampleTask 构造测试用 Task。
func sampleTask(id string) entity.Task {
	now := time.Now()
	return entity.Task{
		ID:        id,
		Type:      entity.TaskTypeAgentRun,
		Input:     `{"task":"hello"}`,
		Status:    valueobject.TaskStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// TestTask_Save_FindByID_RoundTrip 验证：Task 存取往返，Type/Status 字符串映射正确。
func TestTask_Save_FindByID_RoundTrip(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormTaskRepository(db)
	ctx := context.Background()

	task := sampleTask("task-1")
	if err := repo.Save(ctx, task); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}

	got, err := repo.FindByID(ctx, "task-1")
	if err != nil {
		t.Fatalf("FindByID 失败: %v", err)
	}
	if got.Type != entity.TaskTypeAgentRun {
		t.Errorf("Type = %q, want agent_run", got.Type)
	}
	if got.Status != valueobject.TaskStatusPending {
		t.Errorf("Status = %q, want pending", got.Status)
	}
}

// TestTask_FindByID_NotFound 验证：查不存在返回 ErrNotFound（worker 依赖它判断）。
func TestTask_FindByID_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormTaskRepository(db)

	_, err := repo.FindByID(context.Background(), "no-such-task")
	if !errors.Is(err, pkg.ErrNotFound) {
		t.Errorf("应返回 ErrNotFound，得到 %v", err)
	}
}

// TestTask_UpdateStatus_FullStateMachine 验证：任务状态机完整流转。
// pending → running → succeeded（成功路径）
// pending → running → failed（失败路径，含 error 消息）
// 这是 worker.processTask 的核心依赖——状态流转错会导致任务卡死或状态混乱。
func TestTask_UpdateStatus_FullStateMachine(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormTaskRepository(db)
	ctx := context.Background()

	// 成功路径
	_ = repo.Save(ctx, sampleTask("ok-task"))
	mustUpdateStatus(t, repo, ctx, "ok-task", valueobject.TaskStatusRunning, "")
	mustUpdateStatus(t, repo, ctx, "ok-task", valueobject.TaskStatusSucceeded, "")
	got, _ := repo.FindByID(ctx, "ok-task")
	if got.Status != valueobject.TaskStatusSucceeded {
		t.Errorf("成功路径 Status = %q, want succeeded", got.Status)
	}

	// 失败路径（带 error 消息）
	_ = repo.Save(ctx, sampleTask("fail-task"))
	mustUpdateStatus(t, repo, ctx, "fail-task", valueobject.TaskStatusRunning, "")
	mustUpdateStatus(t, repo, ctx, "fail-task", valueobject.TaskStatusFailed, "LLM 调用超时")
	got, _ = repo.FindByID(ctx, "fail-task")
	if got.Status != valueobject.TaskStatusFailed {
		t.Errorf("失败路径 Status = %q, want failed", got.Status)
	}
	if got.Error != "LLM 调用超时" {
		t.Errorf("Error = %q, want 'LLM 调用超时'", got.Error)
	}
}

// TestTask_UpdateStatus_NotFound 验证：更新不存在的任务返回 ErrNotFound。
// 这是 UpdateStatus 的 RowsAffected==0 检查——防止"更新了 0 行却返回成功"。
func TestTask_UpdateStatus_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormTaskRepository(db)

	err := repo.UpdateStatus(context.Background(), "ghost", valueobject.TaskStatusRunning, "")
	if !errors.Is(err, pkg.ErrNotFound) {
		t.Errorf("更新不存在的任务应返回 ErrNotFound，得到 %v", err)
	}
}

// TestTask_UpdateOutput 验证：任务完成后写入输出（Agent 回复）。
func TestTask_UpdateOutput(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormTaskRepository(db)
	ctx := context.Background()

	_ = repo.Save(ctx, sampleTask("out-task"))
	if err := repo.UpdateOutput(ctx, "out-task", `{"response":"done"}`); err != nil {
		t.Fatalf("UpdateOutput 失败: %v", err)
	}
	got, _ := repo.FindByID(ctx, "out-task")
	if got.Output != `{"response":"done"}` {
		t.Errorf("Output = %q", got.Output)
	}
}

// TestTask_UpdateOutput_NotFound 验证：更新不存在任务的输出返回 ErrNotFound。
func TestTask_UpdateOutput_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormTaskRepository(db)

	err := repo.UpdateOutput(context.Background(), "ghost", "x")
	if !errors.Is(err, pkg.ErrNotFound) {
		t.Errorf("应返回 ErrNotFound，得到 %v", err)
	}
}

// TestTask_UpdateProgress 验证：进度更新（前端实时展示"正在采集..."）。
func TestTask_UpdateProgress(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormTaskRepository(db)
	ctx := context.Background()

	_ = repo.Save(ctx, sampleTask("prog-task"))
	if err := repo.UpdateProgress(ctx, "prog-task", "正在采集第 3 页..."); err != nil {
		t.Fatalf("UpdateProgress 失败: %v", err)
	}
	got, _ := repo.FindByID(ctx, "prog-task")
	if got.Progress != "正在采集第 3 页..." {
		t.Errorf("Progress = %q", got.Progress)
	}
}

// TestTask_List 验证：按 created_at DESC 排序列出（任务列表页依赖）。
func TestTask_List(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormTaskRepository(db)
	ctx := context.Background()

	t1 := sampleTask("t1"); t1.CreatedAt = time.Now().Add(-2 * time.Minute)
	t2 := sampleTask("t2"); t2.CreatedAt = time.Now().Add(-1 * time.Minute)
	t3 := sampleTask("t3"); t3.CreatedAt = time.Now()
	_ = repo.Save(ctx, t1)
	_ = repo.Save(ctx, t2)
	_ = repo.Save(ctx, t3)

	list, err := repo.List(ctx, 10)
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("任务数 = %d, want 3", len(list))
	}
	// 最新的排前面（t3 先）
	if list[0].ID != "t3" {
		t.Errorf("最新任务应排首位，得到 %q", list[0].ID)
	}
}

// mustUpdateStatus 更新状态，失败则 t.Fatal。
func mustUpdateStatus(t *testing.T, repo *GormTaskRepository, ctx context.Context, id string, status valueobject.TaskStatus, errMsg string) {
	t.Helper()
	if err := repo.UpdateStatus(ctx, id, status, errMsg); err != nil {
		t.Fatalf("UpdateStatus(%s→%s) 失败: %v", id, status, err)
	}
}
