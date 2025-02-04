package wikipediaclient

type SearchResult struct {
	Title     string `json:"title"`
	PageID    int    `json:"pageid"`
	URL       string `json:"url"`
	Snippet   string `json:"snippet"`
	WordCount int    `json:"wordcount"`
	Language  string `json:"language"`
}
