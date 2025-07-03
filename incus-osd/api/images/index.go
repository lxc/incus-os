package images

// Index represents the content of index.json/index.sjson.
type Index struct {
	Format string `json:"format"`

	Updates []UpdateFull `json:"updates"`
}
