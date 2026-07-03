package model

type Dependency struct {
	File  string `json:"file"`
	Line  int    `json:"line"`
	Uses  string `json:"uses"`
	Owner string `json:"owner,omitempty"`
	Repo  string `json:"repo,omitempty"`
	Path  string `json:"path,omitempty"`
	Ref   string `json:"ref,omitempty"`
}

type Status string

const (
	StatusHealthy     Status = "healthy"
	StatusBlocked     Status = "blocked"
	StatusUnavailable Status = "unavailable"
	StatusDisabled    Status = "disabled"
	StatusArchived    Status = "archived"
	StatusMissingRef  Status = "missing_ref"
	StatusMissingPath Status = "missing_path"
	StatusInvalid     Status = "invalid"
	StatusAPIError    Status = "api_error"
)

type Result struct {
	Dependency Dependency `json:"dependency"`
	Status     Status     `json:"status"`
	HTTPStatus int        `json:"http_status,omitempty"`
	Message    string     `json:"message"`
}

func (r Result) IsFailure() bool {
	return r.Status != StatusHealthy
}

func (r Result) IsOperationalError() bool {
	return r.Status == StatusAPIError
}
