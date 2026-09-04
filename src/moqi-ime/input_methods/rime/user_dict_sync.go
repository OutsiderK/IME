package rime

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/gaboolic/moqi-ime/imecore"
)

type userDictSyncState struct {
	mu      sync.Mutex
	running bool
}

var sharedUserDictSyncState userDictSyncState

func resetUserDictSyncStateForTest() {
	sharedUserDictSyncState.mu.Lock()
	sharedUserDictSyncState.running = false
	sharedUserDictSyncState.mu.Unlock()
}

func (s *userDictSyncState) begin() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return false
	}
	s.running = true
	return true
}

func (s *userDictSyncState) end() {
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
}

// Rime's native sync operation exports the current user database to the local
// sync directory and merges snapshots already placed there. Keeping this local
// preserves the useful import/export path without WebDAV or cloud state.
func (ime *IME) runLocalUserDictionarySync(resp *imecore.Response, successMessage string) bool {
	if ime.backend == nil {
		if resp != nil {
			resp.TrayNotification = trayNotification("个人词库不可用：Rime 尚未启动", imecore.TrayNotificationIconError)
		}
		return false
	}
	if !sharedUserDictSyncState.begin() {
		if resp != nil {
			resp.TrayNotification = trayNotification("个人词库正在处理", imecore.TrayNotificationIconInfo)
		}
		return true
	}

	run := func() *imecore.TrayNotification {
		defer sharedUserDictSyncState.end()
		if !ime.backend.SyncUserData() {
			return trayNotification("个人词库处理失败", imecore.TrayNotificationIconError)
		}
		return trayNotification(successMessage, imecore.TrayNotificationIconInfo)
	}

	if ime.asyncResponseSender == nil {
		notification := run()
		if resp != nil {
			resp.TrayNotification = notification
		}
		return notification.Icon != imecore.TrayNotificationIconError
	}
	if resp != nil {
		resp.TrayNotification = trayNotification("正在处理个人词库…", imecore.TrayNotificationIconInfo)
	}
	go func() { ime.sendAsyncTrayNotification(run()) }()
	return true
}

func (ime *IME) exportUserDictionary(resp *imecore.Response) bool {
	return ime.runLocalUserDictionarySync(resp, "词库快照已导出到本地")
}

func (ime *IME) importUserDictionary(resp *imecore.Response) bool {
	return ime.runLocalUserDictionarySync(resp, "本地词库快照已合并")
}

func (ime *IME) syncUserDataCommand(resp *imecore.Response) bool {
	return ime.importUserDictionary(resp)
}

func (ime *IME) userDictionarySnapshotDir() (string, error) {
	userDir := ime.userDir()
	if userDir == "" {
		return "", fmt.Errorf("无法确定 Rime 用户目录")
	}
	path := filepath.Join(userDir, "sync")
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", err
	}
	return path, nil
}
