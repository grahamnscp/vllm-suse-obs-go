package models

// ai model type
type ModelData struct {
	ID      string      `yaml:"id"`
	Object  string      `yaml:"object"`
	Created int         `yaml:"created"`
	OwnedBy string      `yaml:"owned_by"`
	Root    interface{} `yaml:"root"`
	Parent  interface{} `yaml:"parent"`
}

type ListObject struct {
	Object string      `yaml:"object"`
	Data   []ModelData `yaml:"data"`
}

