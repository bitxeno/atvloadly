package model

import "time"

// SavedSource is an AltStore source the user saved to browse and search its apps.
type SavedSource struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	URL       string    `gorm:"uniqueIndex" json:"url"` // normalized source URL
	Name      string    `json:"name"`                   // source name when it was saved
}
