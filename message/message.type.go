package message

type Message struct {
	Username        string           `json:"username"`
	MessageContents []MessageContent `json:"message_contents"`
}

type MessageContent struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

type EmoticonMapper struct {
	Emoticon string `json:"emoticon"`
	Path     string `json:"path"`
}
