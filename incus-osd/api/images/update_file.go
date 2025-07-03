package images

// UpdateFile represents a file entry in an update.
type UpdateFile struct {
	Architecture UpdateFileArchitecture `json:"architecture"`
	Component    UpdateFileComponent    `json:"component"`
	Filename     string                 `json:"filename"`
	Sha256       string                 `json:"sha256"`
	Size         int64                  `json:"size"`
	Type         UpdateFileType         `json:"type"`
}
