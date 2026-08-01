package services

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"
	"sync"

	"tavily-proxy/server/internal/models"

	"gorm.io/gorm"
)

const (
	defaultAccessKeyName            = "DESKTOP_ACCESS_KEY"
	accessKeysInitializedSettingKey = "access_keys_initialized"
)

type AccessKeyService struct {
	db *gorm.DB

	mu   sync.RWMutex
	keys []accessKeyCredential
}

type accessKeyCredential struct {
	value      string
	restricted bool
}

func NewAccessKeyService(db *gorm.DB) *AccessKeyService {
	return &AccessKeyService{db: db}
}

// Load imports the configured desktop key once, then leaves the collection fully
// managed by the database so deletions remain effective after a restart.
func (s *AccessKeyService) Load(ctx context.Context, desktopAccessKey string) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var initialized models.Setting
		err := tx.First(&initialized, "key = ?", accessKeysInitializedSettingKey).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var count int64
		if err := tx.Model(&models.AccessKey{}).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			key := strings.TrimSpace(desktopAccessKey)
			if key == "" {
				var err error
				key, err = generateSecret(32)
				if err != nil {
					return err
				}
			}
			if err := tx.Create(&models.AccessKey{
				Name:         defaultAccessKeyName,
				Key:          key,
				IsRestricted: true,
			}).Error; err != nil {
				return err
			}
		}
		return tx.Create(&models.Setting{
			Key:   accessKeysInitializedSettingKey,
			Value: "true",
		}).Error
	})
	if err != nil {
		return err
	}
	return s.reload(ctx)
}

func (s *AccessKeyService) List(ctx context.Context) ([]models.AccessKey, error) {
	var keys []models.AccessKey
	if err := s.db.WithContext(ctx).Order("id ASC").Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

func (s *AccessKeyService) Create(ctx context.Context, name string) (*models.AccessKey, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("access key name is required")
	}
	if len(name) > 100 {
		return nil, errors.New("access key name is too long")
	}
	key, err := generateSecret(32)
	if err != nil {
		return nil, err
	}
	item := &models.AccessKey{Name: name, Key: key}
	if err := s.db.WithContext(ctx).Create(item).Error; err != nil {
		return nil, err
	}
	if err := s.reload(ctx); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *AccessKeyService) Delete(ctx context.Context, id uint) error {
	result := s.db.WithContext(ctx).Delete(&models.AccessKey{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return s.reload(ctx)
}

func (s *AccessKeyService) Authenticate(token string) bool {
	return s.authenticate(token, false)
}

func (s *AccessKeyService) AuthenticateRestricted(token string) bool {
	return s.authenticate(token, true)
}

func (s *AccessKeyService) authenticate(token string, restrictedOnly bool) bool {
	if s == nil || token == "" {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, key := range s.keys {
		if restrictedOnly != key.restricted {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(key.value), []byte(token)) == 1 {
			return true
		}
	}
	return false
}

func (s *AccessKeyService) reload(ctx context.Context) error {
	var items []models.AccessKey
	if err := s.db.WithContext(ctx).Select("key", "is_restricted").Find(&items).Error; err != nil {
		return err
	}
	keys := make([]accessKeyCredential, 0, len(items))
	for _, item := range items {
		keys = append(keys, accessKeyCredential{value: item.Key, restricted: item.IsRestricted})
	}
	s.mu.Lock()
	s.keys = keys
	s.mu.Unlock()
	return nil
}
