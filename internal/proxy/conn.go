// internal/proxy/conn.go
func (c *conn) process(ctx context.Context) {
	// 原有连接处理逻辑...
	
	// 用户识别 - 优先通过密钥识别
	if mgr, ok := c.proxy.secretManager.(*secret.DynamicManager); ok {
		// 通过密钥识别用户
		if userID, exists := mgr.GetUserIDBySecret(c.secret.String()); exists {
			c.userID = userID
		} 
		// 通过端口识别用户（如果密钥识别失败）
		else if userID, exists := mgr.GetUserIDByPort(c.listener.Addr().Port); exists {
			c.userID = userID
		}
	}
	
	// 应用用户限速（如果配置了）
	if mgr, ok := c.proxy.secretManager.(*secret.DynamicManager); ok && c.userID != "" {
		if speed, exists := mgr.GetUserSpeed(c.userID); exists && speed > 0 {
			c.speedLimit = speed * 1024 * 1024 // 转换为字节/秒
		}
	}
	
	// 记录流量时关联userID
	c.trafficRecorder = stats.NewTrafficRecorder(c.userID)
	
	// 后续处理...
}

// internal/secret/manager.go
type DynamicManager struct {
	client   *sspanel.Client
	secrets  map[string]string // secret -> userID
	ports    map[int]string    // port -> userID
	speeds   map[string]int    // userID -> 速度限制(Mbps)
	mu       sync.RWMutex
	interval time.Duration
}

func (d *DynamicManager) sync(ctx context.Context) {
	users, err := d.client.FetchUsers(ctx)
	if err != nil {
		return // 记录日志
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	newSecrets := make(map[string]string)
	newPorts := make(map[int]string)
	newSpeeds := make(map[string]int)

	for _, user := range users {
		newSecrets[user.Passwd] = user.ID
		newPorts[user.Port] = user.ID
		newSpeeds[user.ID] = user.Speed
	}

	d.secrets = newSecrets
	d.ports = newPorts
	d.speeds = newSpeeds
}

func (d *DynamicManager) GetUserSpeed(userID string) (int, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	speed, exists := d.speeds[userID]
	return speed, exists
}