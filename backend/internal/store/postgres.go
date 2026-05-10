package store

import (
	"context"
	"database/sql/driver"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const defaultDSN = "host=localhost port=5432 user=admin_go password=admin_go_password dbname=flutter_admin_go sslmode=disable TimeZone=Asia/Shanghai"
const defaultMinIOEndpoint = "localhost:9000"
const defaultMinIOAccessKey = "admin_go"
const defaultMinIOSecretKey = "admin_go_password"
const defaultAvatarBucket = "admin-avatars"

//go:embed migrations/*.sql
var migrationFiles embed.FS

var db *gorm.DB
var objectClient *minio.Client
var avatarBucket string

type IntArray []int

func (ids IntArray) Value() (driver.Value, error) {
	if ids == nil {
		ids = IntArray{}
	}
	b, err := json.Marshal(ids)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (ids *IntArray) Scan(value interface{}) error {
	if value == nil {
		*ids = IntArray{}
		return nil
	}

	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("unsupported IntArray value %T", value)
	}

	if len(raw) == 0 {
		*ids = IntArray{}
		return nil
	}
	return json.Unmarshal(raw, ids)
}

type AdminUser struct {
	ID           int      `gorm:"primaryKey;column:id"`
	Username     string   `gorm:"column:username"`
	Password     string   `gorm:"column:password"`
	Nickname     string   `gorm:"column:nickname"`
	RoleIDs      IntArray `gorm:"column:role_ids;type:jsonb"`
	Theme        string   `gorm:"column:theme"`
	AvatarKey    string   `gorm:"column:avatar_key"`
	ThumbnailKey string   `gorm:"column:thumbnail_key"`
}

func (AdminUser) TableName() string {
	return "admin_users"
}

type AdminRole struct {
	ID      int      `gorm:"primaryKey;column:id"`
	Name    string   `gorm:"column:name"`
	Key     string   `gorm:"column:role_key"`
	MenuIDs IntArray `gorm:"column:menu_ids;type:jsonb"`
}

func (AdminRole) TableName() string {
	return "admin_roles"
}

type AdminMenu struct {
	ID         int    `gorm:"primaryKey;column:id"`
	Name       string `gorm:"column:name"`
	Path       string `gorm:"column:path"`
	ParentID   int    `gorm:"column:parent_id"`
	Type       string `gorm:"column:type"`
	Permission string `gorm:"column:permission"`
}

func (AdminMenu) TableName() string {
	return "admin_menus"
}

type MobileUser struct {
	ID       int    `gorm:"primaryKey;column:id"`
	Username string `gorm:"column:username"`
	Password string `gorm:"column:password"`
	Nickname string `gorm:"column:nickname"`
}

func (MobileUser) TableName() string {
	return "mobile_users"
}

func Init(_ string) error {
	dsn := strings.TrimSpace(os.Getenv("DATABASE_DSN"))
	if dsn == "" {
		dsn = defaultDSN
	}

	conn, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("open postgres: %w", err)
	}

	sqlDB, err := conn.DB()
	if err != nil {
		return fmt.Errorf("get postgres connection: %w", err)
	}
	if err = sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return fmt.Errorf("ping postgres: %w", err)
	}

	db = conn
	if err = migrate(); err != nil {
		return err
	}
	if err = initObjectStore(); err != nil {
		return err
	}
	return nil
}

func DB() *gorm.DB {
	return db
}

func ObjectClient() *minio.Client {
	return objectClient
}

func AvatarBucket() string {
	return avatarBucket
}

func initObjectStore() error {
	endpoint := envOrDefault("MINIO_ENDPOINT", defaultMinIOEndpoint)
	accessKey := envOrDefault("MINIO_ACCESS_KEY", defaultMinIOAccessKey)
	secretKey := envOrDefault("MINIO_SECRET_KEY", defaultMinIOSecretKey)
	avatarBucket = envOrDefault("MINIO_AVATAR_BUCKET", defaultAvatarBucket)
	useSSL := strings.EqualFold(os.Getenv("MINIO_USE_SSL"), "true")

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return fmt.Errorf("create minio client: %w", err)
	}

	ctx := context.Background()
	exists, err := client.BucketExists(ctx, avatarBucket)
	if err != nil {
		return fmt.Errorf("check minio bucket: %w", err)
	}
	if !exists {
		if err = client.MakeBucket(ctx, avatarBucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("create minio bucket: %w", err)
		}
	}
	objectClient = client
	return nil
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func migrate() error {
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`).Error; err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		version := entry.Name()

		var count int64
		if err := db.Table("schema_migrations").Where("version = ?", version).Count(&count).Error; err != nil {
			return fmt.Errorf("check migration %s: %w", version, err)
		}
		if count > 0 {
			continue
		}

		sqlBytes, err := migrationFiles.ReadFile("migrations/" + version)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", version, err)
		}

		err = db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(string(sqlBytes)).Error; err != nil {
				return err
			}
			return tx.Exec(`INSERT INTO schema_migrations(version) VALUES (?)`, version).Error
		})
		if err != nil {
			return fmt.Errorf("apply migration %s: %w", version, err)
		}
	}
	return nil
}
