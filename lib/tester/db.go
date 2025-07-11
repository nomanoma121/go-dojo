package tester

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	_ "github.com/lib/pq"
)

// PostgreSQLContainer represents a test PostgreSQL container
type PostgreSQLContainer struct {
	Pool     *dockertest.Pool
	Resource *dockertest.Resource
	DB       *sql.DB
	DSN      string
}

// SetupPostgreSQL creates a PostgreSQL container for testing
func SetupPostgreSQL(t *testing.T) *PostgreSQLContainer {
	t.Helper()

	pool, err := dockertest.NewPool("")
	if err != nil {
		t.Fatalf("Could not connect to docker: %s", err)
	}

	// Uses pool to try to connect to Docker
	err = pool.Client.Ping()
	if err != nil {
		t.Fatalf("Could not connect to Docker: %s", err)
	}

	// Pulls an image, creates a container based on it and runs it
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "15",
		Env: []string{
			"POSTGRES_PASSWORD=secret",
			"POSTGRES_USER=user",
			"POSTGRES_DB=testdb",
			"listen_addresses = '*'",
		},
	}, func(config *docker.HostConfig) {
		// set AutoRemove to true so that stopped container goes away by itself
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		t.Fatalf("Could not start resource: %s", err)
	}

	hostAndPort := resource.GetHostPort("5432/tcp")
	databaseUrl := fmt.Sprintf("postgres://user:secret@%s/testdb?sslmode=disable", hostAndPort)

	t.Logf("Connecting to database on url: %s", databaseUrl)

	resource.Expire(120) // Tell docker to hard kill the container in 120 seconds

	var db *sql.DB
	// exponential backoff-retry, because the application in the container might not be ready to accept connections yet
	if err := pool.Retry(func() error {
		db, err = sql.Open("postgres", databaseUrl)
		if err != nil {
			return err
		}
		return db.Ping()
	}); err != nil {
		t.Fatalf("Could not connect to docker: %s", err)
	}

	return &PostgreSQLContainer{
		Pool:     pool,
		Resource: resource,
		DB:       db,
		DSN:      databaseUrl,
	}
}

// Close cleans up the PostgreSQL container
func (c *PostgreSQLContainer) Close(t *testing.T) {
	t.Helper()

	if c.DB != nil {
		c.DB.Close()
	}

	if err := c.Pool.Purge(c.Resource); err != nil {
		t.Fatalf("Could not purge resource: %s", err)
	}
}

// RedisContainer represents a test Redis container
type RedisContainer struct {
	Pool     *dockertest.Pool
	Resource *dockertest.Resource
	Address  string
}

// SetupRedis creates a Redis container for testing
func SetupRedis(t *testing.T) *RedisContainer {
	t.Helper()

	pool, err := dockertest.NewPool("")
	if err != nil {
		t.Fatalf("Could not connect to docker: %s", err)
	}

	err = pool.Client.Ping()
	if err != nil {
		t.Fatalf("Could not connect to Docker: %s", err)
	}

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "redis",
		Tag:        "7",
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		t.Fatalf("Could not start resource: %s", err)
	}

	address := resource.GetHostPort("6379/tcp")
	resource.Expire(120)

	return &RedisContainer{
		Pool:     pool,
		Resource: resource,
		Address:  address,
	}
}

// Close cleans up the Redis container
func (c *RedisContainer) Close(t *testing.T) {
	t.Helper()

	if err := c.Pool.Purge(c.Resource); err != nil {
		t.Fatalf("Could not purge resource: %s", err)
	}
}