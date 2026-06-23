package activityapp

import "net/http"

// NewHandler initializes the activity module and returns its HTTP handler for
// embedding inside the unified fufu tool site. The root directory should contain
// the activity public/ and data/ directories. When embedded, the host is the
// source of truth for NewAPI/MCY runtime config, so this path intentionally
// skips environment/bootstrap site loading and waits for the host to apply its
// persisted config snapshot.
func NewHandler(root string) (http.Handler, error) {
	rootDir = root
	if err := initAllEmbedded(); err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/", apiRoute)
	mux.HandleFunc("/", staticRoute)
	return mux, nil
}

// StartWorkers starts background activity jobs for the production server.
// 自动补卡由 DB 任务队列驱动：到点只入队，worker 负责超时取消、库存确认
// 和失败状态持久化。
func StartWorkers() {
	startCreditWorker()
	startSaleCardScheduler()
}

// Close releases activity module resources. It is primarily used by embedded
// server tests on Windows, where open SQLite files block temp-dir cleanup.
func Close() error {
	if db == nil {
		return nil
	}
	err := db.Close()
	db = nil
	return err
}
