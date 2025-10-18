package models

import "time"

type Project struct {
	ID          int
	RepoName    string
	Title       string
	Description string
	ImageURL    string
	GitHubURL   string
	CustomURL   string
	Enabled     bool
	UpdatedAt   time.Time
}
