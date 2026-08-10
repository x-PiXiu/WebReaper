package auth

import (
	"context"
	"errors"
	"testing"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// ---- 假实现 ----

type fakeUserRepo struct {
	users map[string]entity.User // key = username
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{users: make(map[string]entity.User)}
}
func (r *fakeUserRepo) Save(_ context.Context, u entity.User) error {
	r.users[u.Username] = u
	return nil
}
func (r *fakeUserRepo) FindByUsername(_ context.Context, username string) (entity.User, error) {
	u, ok := r.users[username]
	if !ok {
		return entity.User{}, pkg.ErrNotFound
	}
	return u, nil
}
func (r *fakeUserRepo) FindByID(_ context.Context, id string) (entity.User, error) {
	for _, u := range r.users {
		if u.ID == id {
			return u, nil
		}
	}
	return entity.User{}, pkg.ErrNotFound
}
func (r *fakeUserRepo) List(_ context.Context) ([]entity.User, error) {
	out := make([]entity.User, 0, len(r.users))
	for _, u := range r.users {
		out = append(out, u)
	}
	return out, nil
}
func (r *fakeUserRepo) Delete(_ context.Context, id string) error {
	for k, u := range r.users {
		if u.ID == id {
			delete(r.users, k)
			return nil
		}
	}
	return nil
}
func (r *fakeUserRepo) Count(_ context.Context) (int, error) {
	return len(r.users), nil
}

// fakeHasher 简单的哈希：加前缀 "hash:"，Compare 直接字符串比较。
type fakeHasher struct{}

func (fakeHasher) Hash(p string) (string, error) { return "hash:" + p, nil }
func (fakeHasher) Compare(hash, p string) error {
	if hash != "hash:"+p {
		return errors.New("mismatch")
	}
	return nil
}

// fakeTokenGen 假的 token 生成器。
type fakeTokenGen struct{}

func (fakeTokenGen) Generate(c port.TokenClaims) (string, error) {
	return "token-" + c.Username, nil
}

// ---- 注册测试 ----

func TestRegister_Success(t *testing.T) {
	uc := NewRegisterUseCase(newFakeUserRepo(), fakeHasher{})
	out, err := uc.Execute(context.Background(), RegisterInput{Username: "alice", Password: "secret123"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.UserID == "" {
		t.Error("UserID should not be empty")
	}
}

func TestRegister_ShortUsername(t *testing.T) {
	uc := NewRegisterUseCase(newFakeUserRepo(), fakeHasher{})
	_, err := uc.Execute(context.Background(), RegisterInput{Username: "ab", Password: "secret123"})
	if !errors.Is(err, pkg.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestRegister_ShortPassword(t *testing.T) {
	uc := NewRegisterUseCase(newFakeUserRepo(), fakeHasher{})
	_, err := uc.Execute(context.Background(), RegisterInput{Username: "alice", Password: "123"})
	if !errors.Is(err, pkg.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestRegister_Duplicate(t *testing.T) {
	repo := newFakeUserRepo()
	uc := NewRegisterUseCase(repo, fakeHasher{})
	_, _ = uc.Execute(context.Background(), RegisterInput{Username: "alice", Password: "secret123"})

	_, err := uc.Execute(context.Background(), RegisterInput{Username: "alice", Password: "secret456"})
	if !errors.Is(err, pkg.ErrAlreadyExists) {
		t.Errorf("expected ErrAlreadyExists, got %v", err)
	}
}

// ---- 登录测试 ----

func TestLogin_Success(t *testing.T) {
	repo := newFakeUserRepo()
	regUC := NewRegisterUseCase(repo, fakeHasher{})
	_, _ = regUC.Execute(context.Background(), RegisterInput{Username: "alice", Password: "secret123"})

	loginUC := NewLoginUseCase(repo, fakeHasher{}, fakeTokenGen{})
	out, err := loginUC.Execute(context.Background(), LoginInput{Username: "alice", Password: "secret123"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.Token != "token-alice" {
		t.Errorf("Token = %q, want token-alice", out.Token)
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	loginUC := NewLoginUseCase(newFakeUserRepo(), fakeHasher{}, fakeTokenGen{})
	_, err := loginUC.Execute(context.Background(), LoginInput{Username: "ghost", Password: "secret123"})
	if !errors.Is(err, pkg.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument (unified), got %v", err)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	repo := newFakeUserRepo()
	regUC := NewRegisterUseCase(repo, fakeHasher{})
	_, _ = regUC.Execute(context.Background(), RegisterInput{Username: "alice", Password: "secret123"})

	loginUC := NewLoginUseCase(repo, fakeHasher{}, fakeTokenGen{})
	_, err := loginUC.Execute(context.Background(), LoginInput{Username: "alice", Password: "wrong"})
	if !errors.Is(err, pkg.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument (unified), got %v", err)
	}
}
