package main

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "github.com/lib/pq"
)

// TestUserRepository tests the UserRepository interface implementations
func TestUserRepository(t *testing.T) {
	// Test with MockUserRepository
	t.Run("MockUserRepository", func(t *testing.T) {
		testUserRepositoryImplementation(t, NewMockUserRepository())
	})

	// Test with PostgreSQLUserRepository (requires database)
	if testing.Short() {
		t.Skip("Skipping database tests in short mode")
	}

	db := setupTestDatabase(t)
	defer db.Close()

	t.Run("PostgreSQLUserRepository", func(t *testing.T) {
		testUserRepositoryImplementation(t, NewPostgreSQLUserRepository(db))
	})
}

func testUserRepositoryImplementation(t *testing.T, repo UserRepository) {
	ctx := context.Background()

	t.Run("Create and GetByID", func(t *testing.T) {
		user := &User{
			Username: "testuser",
			Email:    "test@example.com",
			Created:  time.Now(),
		}

		err := repo.Create(ctx, user)
		require.NoError(t, err)
		assert.NotZero(t, user.ID)

		retrievedUser, err := repo.GetByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, user.Username, retrievedUser.Username)
		assert.Equal(t, user.Email, retrievedUser.Email)
	})

	t.Run("GetByEmail", func(t *testing.T) {
		user := &User{
			Username: "emailtest",
			Email:    "email@example.com",
			Created:  time.Now(),
		}

		err := repo.Create(ctx, user)
		require.NoError(t, err)

		retrievedUser, err := repo.GetByEmail(ctx, user.Email)
		require.NoError(t, err)
		assert.Equal(t, user.Username, retrievedUser.Username)
		assert.Equal(t, user.Email, retrievedUser.Email)
	})

	t.Run("Update", func(t *testing.T) {
		user := &User{
			Username: "updatetest",
			Email:    "update@example.com",
			Created:  time.Now(),
		}

		err := repo.Create(ctx, user)
		require.NoError(t, err)

		user.Username = "updated"
		user.Email = "updated@example.com"

		err = repo.Update(ctx, user)
		require.NoError(t, err)

		retrievedUser, err := repo.GetByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, "updated", retrievedUser.Username)
		assert.Equal(t, "updated@example.com", retrievedUser.Email)
	})

	t.Run("Delete", func(t *testing.T) {
		user := &User{
			Username: "deletetest",
			Email:    "delete@example.com",
			Created:  time.Now(),
		}

		err := repo.Create(ctx, user)
		require.NoError(t, err)

		err = repo.Delete(ctx, user.ID)
		require.NoError(t, err)

		_, err = repo.GetByID(ctx, user.ID)
		assert.Error(t, err)
	})

	t.Run("List with pagination", func(t *testing.T) {
		// Create multiple users
		for i := 0; i < 5; i++ {
			user := &User{
				Username: "listtest" + string(rune('0'+i)),
				Email:    "list" + string(rune('0'+i)) + "@example.com",
				Created:  time.Now(),
			}
			err := repo.Create(ctx, user)
			require.NoError(t, err)
		}

		// Test pagination
		users, err := repo.List(ctx, 3, 0)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(users), 3)

		users, err = repo.List(ctx, 2, 2)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(users), 2)
	})

	t.Run("FindBySpec", func(t *testing.T) {
		user := &User{
			Username: "spectest",
			Email:    "spec@example.com",
			Created:  time.Now(),
		}

		err := repo.Create(ctx, user)
		require.NoError(t, err)

		spec := UserByEmailSpec{Email: "spec@example.com"}
		users, err := repo.FindBySpec(ctx, spec)
		require.NoError(t, err)
		assert.Len(t, users, 1)
		assert.Equal(t, "spectest", users[0].Username)
	})
}

// TestPostRepository tests the PostRepository interface implementations
func TestPostRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database tests in short mode")
	}

	db := setupTestDatabase(t)
	defer db.Close()

	repo := NewPostgreSQLPostRepository(db)
	userRepo := NewPostgreSQLUserRepository(db)
	ctx := context.Background()

	// Create a test user first
	user := &User{
		Username: "postowner",
		Email:    "owner@example.com",
		Created:  time.Now(),
	}
	err := userRepo.Create(ctx, user)
	require.NoError(t, err)

	t.Run("Create and GetByID", func(t *testing.T) {
		post := &Post{
			UserID:  user.ID,
			Title:   "Test Post",
			Content: "Content of test post",
			Created: time.Now(),
		}

		err := repo.Create(ctx, post)
		require.NoError(t, err)
		assert.NotZero(t, post.ID)

		retrievedPost, err := repo.GetByID(ctx, post.ID)
		require.NoError(t, err)
		assert.Equal(t, post.Title, retrievedPost.Title)
		assert.Equal(t, post.Content, retrievedPost.Content)
		assert.Equal(t, post.UserID, retrievedPost.UserID)
	})

	t.Run("GetByUserID", func(t *testing.T) {
		post1 := &Post{
			UserID:  user.ID,
			Title:   "User Post 1",
			Content: "Content 1",
			Created: time.Now(),
		}
		post2 := &Post{
			UserID:  user.ID,
			Title:   "User Post 2",
			Content: "Content 2",
			Created: time.Now(),
		}

		err := repo.Create(ctx, post1)
		require.NoError(t, err)
		err = repo.Create(ctx, post2)
		require.NoError(t, err)

		posts, err := repo.GetByUserID(ctx, user.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(posts), 2)
	})

	t.Run("Update", func(t *testing.T) {
		post := &Post{
			UserID:  user.ID,
			Title:   "Original Title",
			Content: "Original Content",
			Created: time.Now(),
		}

		err := repo.Create(ctx, post)
		require.NoError(t, err)

		post.Title = "Updated Title"
		post.Content = "Updated Content"

		err = repo.Update(ctx, post)
		require.NoError(t, err)

		retrievedPost, err := repo.GetByID(ctx, post.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated Title", retrievedPost.Title)
		assert.Equal(t, "Updated Content", retrievedPost.Content)
	})

	t.Run("Delete", func(t *testing.T) {
		post := &Post{
			UserID:  user.ID,
			Title:   "To Delete",
			Content: "Will be deleted",
			Created: time.Now(),
		}

		err := repo.Create(ctx, post)
		require.NoError(t, err)

		err = repo.Delete(ctx, post.ID)
		require.NoError(t, err)

		_, err = repo.GetByID(ctx, post.ID)
		assert.Error(t, err)
	})

	t.Run("List with pagination", func(t *testing.T) {
		posts, err := repo.List(ctx, 5, 0)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(posts), 5)
	})
}

// TestUserService tests the UserService business logic
func TestUserService(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database tests in short mode")
	}

	db := setupTestDatabase(t)
	defer db.Close()

	userRepo := NewPostgreSQLUserRepository(db)
	postRepo := NewPostgreSQLPostRepository(db)
	service := NewUserService(userRepo, postRepo, db)
	ctx := context.Background()

	t.Run("CreateUserWithProfile", func(t *testing.T) {
		user := &User{
			Username: "profileuser",
			Email:    "profile@example.com",
			Created:  time.Now(),
		}

		err := service.CreateUserWithProfile(ctx, user, "User bio")
		require.NoError(t, err)
		assert.NotZero(t, user.ID)

		// Verify user was created
		retrievedUser, err := userRepo.GetByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, user.Username, retrievedUser.Username)
	})

	t.Run("CreateUserWithPost", func(t *testing.T) {
		user := &User{
			Username: "postuser",
			Email:    "postuser@example.com",
			Created:  time.Now(),
		}

		post := &Post{
			Title:   "First Post",
			Content: "User's first post",
			Created: time.Now(),
		}

		err := service.CreateUserWithPost(ctx, user, post)
		require.NoError(t, err)
		assert.NotZero(t, user.ID)
		assert.NotZero(t, post.ID)
		assert.Equal(t, user.ID, post.UserID)

		// Verify both user and post were created
		retrievedUser, err := userRepo.GetByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, user.Username, retrievedUser.Username)

		retrievedPost, err := postRepo.GetByID(ctx, post.ID)
		require.NoError(t, err)
		assert.Equal(t, post.Title, retrievedPost.Title)
		assert.Equal(t, user.ID, retrievedPost.UserID)
	})

	t.Run("Transaction rollback on failure", func(t *testing.T) {
		user := &User{
			Username: "",  // Invalid username to cause failure
			Email:    "invalid@example.com",
			Created:  time.Now(),
		}

		post := &Post{
			Title:   "Should not be created",
			Content: "This post should not exist",
			Created: time.Now(),
		}

		err := service.CreateUserWithPost(ctx, user, post)
		assert.Error(t, err)

		// Verify neither user nor post were created
		assert.Zero(t, user.ID)
		assert.Zero(t, post.ID)
	})
}

// TestUnitOfWork tests the Unit of Work pattern
func TestUnitOfWork(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database tests in short mode")
	}

	db := setupTestDatabase(t)
	defer db.Close()

	ctx := context.Background()

	t.Run("Successful transaction", func(t *testing.T) {
		uow := NewUnitOfWork(db)

		err := uow.Begin(ctx)
		require.NoError(t, err)

		user := &User{
			Username: "uowuser",
			Email:    "uow@example.com",
			Created:  time.Now(),
		}

		err = uow.Users().Create(ctx, user)
		require.NoError(t, err)

		post := &Post{
			UserID:  user.ID,
			Title:   "UoW Post",
			Content: "Created in unit of work",
			Created: time.Now(),
		}

		err = uow.Posts().Create(ctx, post)
		require.NoError(t, err)

		err = uow.Commit()
		require.NoError(t, err)

		// Verify data persisted
		userRepo := NewPostgreSQLUserRepository(db)
		retrievedUser, err := userRepo.GetByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, user.Username, retrievedUser.Username)
	})

	t.Run("Failed transaction rollback", func(t *testing.T) {
		uow := NewUnitOfWork(db)

		err := uow.Begin(ctx)
		require.NoError(t, err)

		user := &User{
			Username: "rollbackuser",
			Email:    "rollback@example.com",
			Created:  time.Now(),
		}

		err = uow.Users().Create(ctx, user)
		require.NoError(t, err)

		// Rollback transaction
		err = uow.Rollback()
		require.NoError(t, err)

		// Verify data was not persisted
		userRepo := NewPostgreSQLUserRepository(db)
		_, err = userRepo.GetByID(ctx, user.ID)
		assert.Error(t, err)
	})
}

// TestSpecificationPattern tests the Specification pattern implementations
func TestSpecificationPattern(t *testing.T) {
	t.Run("UserByEmailSpec", func(t *testing.T) {
		spec := UserByEmailSpec{Email: "test@example.com"}
		sql, args := spec.ToSQL()
		
		assert.Contains(t, sql, "email")
		assert.Contains(t, args, "test@example.com")
	})

	t.Run("UserCreatedAfterSpec", func(t *testing.T) {
		after := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
		spec := UserCreatedAfterSpec{After: after}
		sql, args := spec.ToSQL()
		
		assert.Contains(t, sql, "created")
		assert.Contains(t, args, after)
	})

	t.Run("AndSpec", func(t *testing.T) {
		emailSpec := UserByEmailSpec{Email: "test@example.com"}
		dateSpec := UserCreatedAfterSpec{After: time.Now()}
		andSpec := AndSpec{Left: emailSpec, Right: dateSpec}
		
		sql, args := andSpec.ToSQL()
		
		assert.Contains(t, sql, "AND")
		assert.Len(t, args, 2)
	})

	t.Run("OrSpec", func(t *testing.T) {
		emailSpec1 := UserByEmailSpec{Email: "test1@example.com"}
		emailSpec2 := UserByEmailSpec{Email: "test2@example.com"}
		orSpec := OrSpec{Left: emailSpec1, Right: emailSpec2}
		
		sql, args := orSpec.ToSQL()
		
		assert.Contains(t, sql, "OR")
		assert.Len(t, args, 2)
	})

	t.Run("Complex specification combination", func(t *testing.T) {
		emailSpec := UserByEmailSpec{Email: "test@example.com"}
		dateSpec := UserCreatedAfterSpec{After: time.Now()}
		
		// (email = 'test@example.com' AND created > now) OR email = 'other@example.com'
		andSpec := AndSpec{Left: emailSpec, Right: dateSpec}
		otherEmailSpec := UserByEmailSpec{Email: "other@example.com"}
		complexSpec := OrSpec{Left: andSpec, Right: otherEmailSpec}
		
		sql, args := complexSpec.ToSQL()
		
		assert.Contains(t, sql, "AND")
		assert.Contains(t, sql, "OR")
		assert.Len(t, args, 3)
	})
}

// TestTransactionHandling tests transaction scenarios
func TestTransactionHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database tests in short mode")
	}

	db := setupTestDatabase(t)
	defer db.Close()

	ctx := context.Background()

	t.Run("WithTx returns transaction-aware repository", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer tx.Rollback()

		userRepo := NewPostgreSQLUserRepository(db)
		txUserRepo := userRepo.WithTx(tx)

		user := &User{
			Username: "txuser",
			Email:    "tx@example.com",
			Created:  time.Now(),
		}

		err = txUserRepo.Create(ctx, user)
		require.NoError(t, err)

		// Should be visible within transaction
		retrievedUser, err := txUserRepo.GetByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, user.Username, retrievedUser.Username)

		// Rollback - user should not be persisted
		err = tx.Rollback()
		require.NoError(t, err)

		// Should not be visible after rollback
		_, err = userRepo.GetByID(ctx, user.ID)
		assert.Error(t, err)
	})
}

// TestRepositoryErrorHandling tests error scenarios
func TestRepositoryErrorHandling(t *testing.T) {
	t.Run("MockRepository handles not found errors", func(t *testing.T) {
		repo := NewMockUserRepository()
		ctx := context.Background()

		_, err := repo.GetByID(ctx, 999)
		assert.Error(t, err)

		_, err = repo.GetByEmail(ctx, "notfound@example.com")
		assert.Error(t, err)
	})

	t.Run("Repository handles invalid input", func(t *testing.T) {
		repo := NewMockUserRepository()
		ctx := context.Background()

		// Test with nil user
		err := repo.Create(ctx, nil)
		assert.Error(t, err)

		// Test with invalid email
		user := &User{
			Username: "test",
			Email:    "", // Empty email
			Created:  time.Now(),
		}
		err = repo.Create(ctx, user)
		assert.Error(t, err)
	})
}

// Benchmark tests
func BenchmarkUserRepository_Create(b *testing.B) {
	repo := NewMockUserRepository()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		user := &User{
			Username: "benchuser",
			Email:    "bench@example.com",
			Created:  time.Now(),
		}
		repo.Create(ctx, user)
	}
}

func BenchmarkUserRepository_GetByID(b *testing.B) {
	repo := NewMockUserRepository()
	ctx := context.Background()

	// Setup
	user := &User{
		Username: "benchuser",
		Email:    "bench@example.com",
		Created:  time.Now(),
	}
	repo.Create(ctx, user)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		repo.GetByID(ctx, user.ID)
	}
}

// Helper functions

func setupTestDatabase(t *testing.T) *sql.DB {
	// This would typically connect to a test database
	// For this example, we'll use a minimal setup
	db, err := sql.Open("postgres", "postgres://test:test@localhost/testdb?sslmode=disable")
	if err != nil {
		t.Skip("Database not available:", err)
	}

	if err := db.Ping(); err != nil {
		t.Skip("Database not reachable:", err)
	}

	// Setup schema
	err = setupDatabase(db)
	if err != nil {
		t.Fatal("Failed to setup database:", err)
	}

	return db
}

// Table-driven tests for comprehensive coverage

func TestUserRepository_TableDriven(t *testing.T) {
	repo := NewMockUserRepository()
	ctx := context.Background()

	tests := []struct {
		name    string
		user    *User
		wantErr bool
	}{
		{
			name: "valid user",
			user: &User{
				Username: "validuser",
				Email:    "valid@example.com",
				Created:  time.Now(),
			},
			wantErr: false,
		},
		{
			name: "empty username",
			user: &User{
				Username: "",
				Email:    "empty@example.com",
				Created:  time.Now(),
			},
			wantErr: true,
		},
		{
			name: "empty email",
			user: &User{
				Username: "user",
				Email:    "",
				Created:  time.Now(),
			},
			wantErr: true,
		},
		{
			name: "invalid email format",
			user: &User{
				Username: "user",
				Email:    "invalid-email",
				Created:  time.Now(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Create(ctx, tt.user)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotZero(t, tt.user.ID)
			}
		})
	}
}

func TestSpecificationPattern_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		spec UserSpecification
		want struct {
			containsSQL string
			argCount    int
		}
	}{
		{
			name: "email specification",
			spec: UserByEmailSpec{Email: "test@example.com"},
			want: struct {
				containsSQL string
				argCount    int
			}{
				containsSQL: "email",
				argCount:    1,
			},
		},
		{
			name: "date specification",
			spec: UserCreatedAfterSpec{After: time.Now()},
			want: struct {
				containsSQL string
				argCount    int
			}{
				containsSQL: "created",
				argCount:    1,
			},
		},
		{
			name: "and specification",
			spec: AndSpec{
				Left:  UserByEmailSpec{Email: "test@example.com"},
				Right: UserCreatedAfterSpec{After: time.Now()},
			},
			want: struct {
				containsSQL string
				argCount    int
			}{
				containsSQL: "AND",
				argCount:    2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args := tt.spec.ToSQL()
			assert.Contains(t, sql, tt.want.containsSQL)
			assert.Len(t, args, tt.want.argCount)
		})
	}
}