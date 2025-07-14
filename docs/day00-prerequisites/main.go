//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// TODO: 実践課題1を実装してください
// 
// 以下の仕様でHTTP APIを実装してください：
// - GET /users - ユーザー一覧取得
// - POST /users - ユーザー作成  
// - GET /users/{id} - 特定ユーザー取得
// - PUT /users/{id} - ユーザー更新
// - DELETE /users/{id} - ユーザー削除
//
// 要件:
// 1. メモリ内でのデータ保存（スライス使用）
// 2. 適切なHTTPステータスコード
// 3. JSON形式のレスポンス
// 4. エラーハンドリング
// 5. 基本的なバリデーション

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type UserStore struct {
	// TODO: ユーザーデータを保存するためのフィールドを定義
	// ヒント: map[int]*User と sync.RWMutex を使用
}

func NewUserStore() *UserStore {
	// TODO: UserStore を初期化
	return nil
}

func (s *UserStore) CreateUser(user *User) *User {
	// TODO: 新しいユーザーを作成
	// ヒント: IDを自動採番し、mapに保存
	return nil
}

func (s *UserStore) GetUser(id int) (*User, bool) {
	// TODO: 指定されたIDのユーザーを取得
	return nil, false
}

func (s *UserStore) GetAllUsers() []*User {
	// TODO: 全ユーザーを取得
	return nil
}

func (s *UserStore) UpdateUser(id int, user *User) (*User, bool) {
	// TODO: 指定されたIDのユーザーを更新
	return nil, false
}

func (s *UserStore) DeleteUser(id int) bool {
	// TODO: 指定されたIDのユーザーを削除
	return false
}

type UserHandler struct {
	store *UserStore
}

func NewUserHandler() *UserHandler {
	return &UserHandler{
		store: NewUserStore(),
	}
}

func (h *UserHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// TODO: HTTPリクエストを処理
	// ヒント: URLパスとHTTPメソッドに基づいて適切なハンドラーを呼び出す
	
	w.Header().Set("Content-Type", "application/json")
	
	// URLパスから /users を除去
	path := strings.TrimPrefix(r.URL.Path, "/users")
	
	switch r.Method {
	case http.MethodGet:
		// TODO: GET リクエストの処理
	case http.MethodPost:
		// TODO: POST リクエストの処理
	case http.MethodPut:
		// TODO: PUT リクエストの処理
	case http.MethodDelete:
		// TODO: DELETE リクエストの処理
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *UserHandler) handleGetUsers(w http.ResponseWriter, r *http.Request) {
	// TODO: 全ユーザー取得の実装
}

func (h *UserHandler) handleGetUser(w http.ResponseWriter, r *http.Request, path string) {
	// TODO: 特定ユーザー取得の実装
	// ヒント: pathからIDを抽出し、strconv.Atoi()でintに変換
}

func (h *UserHandler) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	// TODO: ユーザー作成の実装
	// ヒント: json.NewDecoder(r.Body).Decode()でリクエストボディを解析
}

func (h *UserHandler) handleUpdateUser(w http.ResponseWriter, r *http.Request, path string) {
	// TODO: ユーザー更新の実装
}

func (h *UserHandler) handleDeleteUser(w http.ResponseWriter, r *http.Request, path string) {
	// TODO: ユーザー削除の実装
}

func main() {
	handler := NewUserHandler()
	http.Handle("/users", handler)
	http.Handle("/users/", handler)
	
	fmt.Println("Server starting on :8080")
	fmt.Println("Try the following endpoints:")
	fmt.Println("  GET    /users")
	fmt.Println("  POST   /users")
	fmt.Println("  GET    /users/{id}")
	fmt.Println("  PUT    /users/{id}")
	fmt.Println("  DELETE /users/{id}")
	
	log.Fatal(http.ListenAndServe(":8080", nil))
}