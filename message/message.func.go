package message

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type messageParser struct {
	emoticonRegex   *regexp.Regexp
	emoticonMappers map[string]string
}

func (mp *messageParser) ParseEmoticon(message string) (messageParsed string) {
	t1 := time.Now()
	ems := mp.emoticonRegex.FindAllString(message, -1)

	messageParsed = message
	//parse emoticon to its emoticon mapper
	for _, em := range ems {
		if val, ok := mp.emoticonMappers[strings.Replace(em, ":", "", -1)]; ok {
			imageURL := `<img src="http://172.31.5.228:8080/` + val + `"width=20 height=20 alt="emoticon_` + em + `"/>`
			messageParsed = strings.Replace(messageParsed, em, imageURL, -1)
		}
	}
	t2 := time.Now()
	fmt.Println(t2.Sub(t1))
	return
}

func NewMessageParser() *messageParser {
	//read from emojis dir
	//emoticon dir relative to caller function
	emDir := "public/emojis/"
	emoticonMappers := make(map[string]string)

	err := filepath.Walk(emDir, func(path string, info os.FileInfo, err error) error {
		//trim filename extension to be used as emoji name
		emojiName := strings.TrimSuffix(info.Name(), filepath.Ext(info.Name()))
		emoticonMappers[emojiName] = path
		return err
	})
	if err != nil {
		panic(err)
	}

	//Init emoticon regex
	emoticonRegex, err := regexp.Compile(":\\S+:")
	if err != nil {
		panic(err)
	}
	return &messageParser{
		emoticonRegex,
		emoticonMappers,
	}
}
