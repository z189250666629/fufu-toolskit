package activityapp

import "net/http"

// NewHandler initializes the activity module and returns its HTTP handler for
// embedding inside the unified fufu tool site. The root directory should contain
// the activity public/ and data/ directories.
func NewHandler(root string) (http.Handler, error) {
	rootDir = root
	if err := initAll(); err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/", apiRoute)
	mux.HandleFunc("/", staticRoute)
	return mux, nil
}

// StartWorkers starts background activity jobs for the production server.
// 自动补卡/MCY 库存检测暂时下线：这里只保留卡密核销后的补额度 worker，
// 不再启动 sale-card scheduler。
func StartWorkers() {
	startCreditWorker()
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
