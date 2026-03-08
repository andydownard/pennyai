package main

import (
	"net/http"
)

// handleCategories handles GET /api/categories — returns all categories
// grouped by parent_category. No auth required (brands and admins both need it).
func (s *Server) handleCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	rows, err := s.db.Query("SELECT id, name, parent_category FROM categories ORDER BY parent_category, name")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load categories"})
		return
	}
	defer rows.Close()

	type Category struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	grouped := make(map[string][]Category)
	for rows.Next() {
		var id int
		var name, parent string
		if err := rows.Scan(&id, &name, &parent); err != nil {
			continue
		}
		grouped[parent] = append(grouped[parent], Category{ID: id, Name: name})
	}

	w.Header().Set("Cache-Control", "public, max-age=3600")
	writeJSON(w, http.StatusOK, grouped)
}
