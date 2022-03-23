package types

// Language represents the language of a File.
type Language struct {
	Confidence string  `json:"confidence"`
	Language   string  `json:"language"`
	RawScore   float64 `json:"rawScore"`
}

// Metadata represents metadata for a File.
type Metadata map[string]interface{}

// NSFWClassification represents the classification returned by nsfw-server.
type NSFWClassification struct {
	Neutral float64 `json:"neutral"`
	Drawing float64 `json:"drawing"`
	Porn    float64 `json:"porn"`
	Hentai  float64 `json:"hentai"`
	Sexy    float64 `json:"sexy"`
}

// NSFW represents nsfw-server classification.
type NSFW struct {
	Classification NSFWClassification `json:"classification"`
	NSFWVersion    string             `json:"nsfwjsVersion"`
}

// File represents a file resource in an Index.
type File struct {
	Document

	Content         string   `json:"content"`
	IpfsTikaVersion string   `json:"ipfs_tika_version"`
	Language        Language `json:"language"`
	Metadata        Metadata `json:"metadata"`
	URLs            []string `json:"urls"`
	NSFW            NSFW     `json:"nfsw"`
}
