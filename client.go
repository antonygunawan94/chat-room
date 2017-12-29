package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Shopify/sarama"
	"github.com/gocql/gocql"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	channelName string

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte

	//Cassandra Cluster
	cluster *gocql.ClusterConfig
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Return.Successes = true
	producer, err := sarama.NewSyncProducer([]string{"127.0.0.1:29092"}, config)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := producer.Close(); err != nil {
			panic(err)
		}
	}()

	defer func() {
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Printf("error: %v", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))

		msg := &sarama.ProducerMessage{
			Topic: c.channelName,
			Value: sarama.StringEncoder(message),
		}
		_, _, errProducer := producer.SendMessage(msg)
		if errProducer != nil {
			panic(errProducer)
		}

		session, err := c.cluster.CreateSession()
		defer session.Close()
		if err != nil {
			panic(err)
			return
		}

		chatMessage := struct {
			Username string `json:"username"`
			Message  string `json:"message"`
		}{}

		err = json.Unmarshal(message, &chatMessage)
		if err != nil {
			log.Println(err)
			return
		}

		err = session.Query(
			"insert into chats(channel, username, message, created_at) VALUES (?, ?, ?, dateof(now()))",
			c.channelName,
			chatMessage.Username,
			chatMessage.Message,
		).Exec()
		if err != nil {
			log.Println(err)
			return
		}
	}
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	consumer, err := sarama.NewConsumer([]string{"127.0.0.1:29092"}, nil)
	if err != nil {
		panic(err)
	}

	defer func() {
		if err := consumer.Close(); err != nil {
			log.Fatalln(err)
		}
	}()

	partitionConsumer, err := consumer.ConsumePartition(c.channelName, 0, sarama.OffsetNewest)
	if err != nil {
		panic(err)
	}

	defer func() {
		if err := partitionConsumer.Close(); err != nil {
			log.Fatalln(err)
		}
	}()

	// Trap SIGINT to trigger a shutdown.
	// signals := make(chan os.Signal, 1)
	// signal.Notify(signals, os.Interrupt)

	for {
		select {
		case err := <-partitionConsumer.Errors():
			fmt.Println(err)
		case msg := <-partitionConsumer.Messages():
			// fmt.Println("Received messages", string(msg.Key), string(msg.Value))
			c.send <- msg.Value
		// case <-signals:
		// 	fmt.Println("Interrupt is detected")
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}

			chatMessage := struct {
				Username string `json:"username"`
				Message  string `json:"message"`
			}{}

			err = json.Unmarshal(message, &chatMessage)
			if err != nil {
				log.Println(err)
				return
			}

			chatMessage.Message = parser.ParseEmoticon(chatMessage.Message)

			bsResponse, err := json.Marshal(chatMessage)
			log.Println(string(bsResponse))
			w.Write(bsResponse)
			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}

}

// serveWs handles websocket requests from the peer.
func ServeWs(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	channelName := r.URL.Query().Get("channel")

	cluster := gocql.NewCluster("172.17.0.2")
	cluster.Keyspace = "chat"

	client := &Client{
		channelName: channelName,
		conn:        conn,
		send:        make(chan []byte, 256),
		cluster:     cluster,
	}

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()
}

/*

CREATE TABLE chat.chats (
    channel text,
    created_at timestamp,
    message text,
    username text,
    PRIMARY KEY (channel, created_at)
)

insert into chats(channel, username, message, created_at) VALUES ('asd', 'antony', 'hehe', dateof(now()));

*/
