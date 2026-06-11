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
