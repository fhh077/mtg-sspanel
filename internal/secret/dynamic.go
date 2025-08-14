package secret

import (
	"context"
	"sync"
	"time"

	"github.com/9seconds/mtg/v2/mtglib"
	"github.com/9seconds/mtg/v2/sspanel"
)

type DynamicManager struct {
	client   *sspanel.Client
	interval time.Duration
	logger   mtglib.Logger

	mu      sync.RWMutex
	secrets map[string]mtglib.Secret // userID -> secret
	ports   map[int]string           // port -> userID
	speeds  map[string]int           // userID -> speed limit
}

func NewDynamicManager(client *sspanel.Client, interval time.Duration, logger mtglib.Logger) *DynamicManager {
	return &DynamicManager{
		client:   client,
		interval: interval,
		logger:   logger,
		secrets:  make(map[string]mtglib.Secret),
		ports:    make(map[int]string),
		speeds:   make(map[string]int),
	}
}

func (d *DynamicManager) GetUserIDBySecret(secret string) (string, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	for userID, s := range d.secrets {
		if s.Hex() == secret || s.Base64() == secret {
			return userID, true
		}
	}
	return "", false
}

func (d *DynamicManager) GetUserIDByPort(port int) (string, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	userID, exists := d.ports[port]
	return userID, exists
}

func (d *DynamicManager) GetUserSpeed(userID string) (int, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	speed, exists := d.speeds[userID]
	return speed, exists
}

func (d *DynamicManager) UserCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.secrets)
}

func (d *DynamicManager) Sync(ctx context.Context) error {
	users, err := d.client.FetchUsers(ctx)
	if err != nil {
		return err
	}

	newSecrets := make(map[string]mtglib.Secret)
	newPorts := make(map[int]string)
	newSpeeds := make(map[string]int)

	for _, user := range users {
		secret, err := mtglib.ParseSecret(user.Passwd)
		if err != nil {
			d.logger.Warn("Invalid secret for user", "user_id", user.ID, "error", err)
			continue
		}

		newSecrets[user.ID] = secret
		newPorts[user.Port] = user.ID
		newSpeeds[user.ID] = user.Speed
	}

	d.mu.Lock()
	d.secrets = newSecrets
	d.ports = newPorts
	d.speeds = newSpeeds
	d.mu.Unlock()

	return nil
}

// 实现 mtglib.Secret 接口
func (d *DynamicManager) Valid() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.secrets) > 0
}

func (d *DynamicManager) String() string {
	return "[dynamic secret manager]"
}

func (d *DynamicManager) Base64() string {
	return ""
}

func (d *DynamicManager) Hex() string {
	return ""
}

func (d *DynamicManager) makeBytes() []byte {
	return nil
}

func (d *DynamicManager) Host() string {
	// 返回第一个有效的域名（所有用户共享同一个域名）
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, secret := range d.secrets {
		return secret.Host
	}
	return ""
}