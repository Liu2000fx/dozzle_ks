package docker

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type EventGenerator struct {
	Events chan *LogEvent
	Errors chan error
	reader *bufio.Reader
	next   *LogEvent
	buffer chan *LogEvent
	tty    bool
	warnNum int
	infoNum int
	errorNum int
	wg     sync.WaitGroup
}

var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

var BadHeaderErr = fmt.Errorf("dozzle/docker: unable to read header")

func NewEventGenerator(reader io.Reader, tty bool) *EventGenerator {
	generator := &EventGenerator{
		reader: bufio.NewReader(reader),
		buffer: make(chan *LogEvent, 100),
		Errors: make(chan error, 1),
		Events: make(chan *LogEvent),
		tty:    tty,
		infoNum: 0,
		warnNum: 0,
		errorNum: 0,
	}
	generator.wg.Add(2)
	go generator.consumeReader()		// 用于消费数据源的读取器lfx
	go generator.processBuffer()		// 用于处理数据缓冲区 lfx
	return generator
}

func (g *EventGenerator) processBuffer() {
	var current, next *LogEvent

	for {
		if g.next != nil {
			current = g.next
			g.next = nil
			next = g.peek()
		} else {
			event, ok := <-g.buffer
			if !ok {
				close(g.Events)
				break
			}

			current = event
			next = g.peek()
		}

		checkPosition(current, next)
		if current.Level == "info" {
			g.infoNum ++
		}else if current.Level == "warn" {
			g.warnNum ++
		}else if current.Level == "error" {
			g.errorNum ++
		}

		g.Events <- current
	}
	g.wg.Done()
}

func (g *EventGenerator) consumeReader() {
	for {
		message, streamType, readerError := readEvent(g.reader, g.tty)
		if message != "" {
			logEvent := createEvent(message, streamType)
			logEvent.Level = guessLogLevel(logEvent)
			
			g.buffer <- logEvent
		}

		if readerError != nil {
			if readerError != BadHeaderErr {
				g.Errors <- readerError
			}
			close(g.buffer)
			break
		}
	}
	g.wg.Done()
}

func (g *EventGenerator) peek() *LogEvent {
	if g.next != nil {
		return g.next
	}
	select {
	case event := <-g.buffer:
		g.next = event
		return g.next
	case <-time.After(50 * time.Millisecond):
		return nil
	}
}

func readEvent(reader *bufio.Reader, tty bool) (string, StdType, error) {
	header := []byte{0, 0, 0, 0, 0, 0, 0, 0}
	buffer := bufPool.Get().(*bytes.Buffer)
	buffer.Reset()
	defer bufPool.Put(buffer)
	var streamType StdType = STDOUT
	if tty {
		message, err := reader.ReadString('\n')
		if err != nil {
			return message, streamType, err
		}
		return message, streamType, nil
	} else {
		n, err := reader.Read(header)
		if err != nil {
			return "", streamType, err
		}
		if n != 8 {
			return "", streamType, BadHeaderErr
		}

		switch header[0] {
		case 1:
			streamType = STDOUT
		case 2:
			streamType = STDERR
		default:
			log.Warnf("unknown stream type %d", header[0])
		}

		count := binary.BigEndian.Uint32(header[4:])
		if count == 0 {
			return "", streamType, nil
		}
		_, err = io.CopyN(buffer, reader, int64(count))
		if err != nil {
			return "", streamType, err
		}
		return buffer.String(), streamType, nil
	}
}

func createEvent(message string, streamType StdType) *LogEvent {
	h := fnv.New32a()
	h.Write([]byte(message))
	logEvent := &LogEvent{Id: h.Sum32(), Message: message, Stream: streamType.String()}
	if index := strings.IndexAny(message, " "); index != -1 {
		logId := message[:index]
		if timestamp, err := time.Parse(time.RFC3339Nano, logId); err == nil {
			logEvent.Timestamp = timestamp.UnixMilli()
			message = strings.TrimSuffix(message[index+1:], "\n")
			logEvent.Message = message
			if json.Valid([]byte(message)) {
				var data map[string]interface{}
				if err := json.Unmarshal([]byte(message), &data); err != nil {
					log.Warnf("unable to parse json logs - error was \"%v\" while trying unmarshal \"%v\"", err.Error(), message)
				} else {
					logEvent.Message = data
				}
			}
		}
	}
	return logEvent
}

func checkPosition(currentEvent *LogEvent, nextEvent *LogEvent) {
	currentLevel := guessLogLevel(currentEvent)
	if nextEvent != nil {
		if currentEvent.IsCloseToTime(nextEvent) && currentLevel != "" && !nextEvent.HasLevel() {
			currentEvent.Position = START
			nextEvent.Position = MIDDLE
		}

		// If next item is not close to current item or has level, set current item position to end
		if currentEvent.Position == MIDDLE && (nextEvent.HasLevel() || !currentEvent.IsCloseToTime(nextEvent)) {
			currentEvent.Position = END
		}

		// If next item is close to current item and has no level, set next item position to middle
		if currentEvent.Position == MIDDLE && !nextEvent.HasLevel() && currentEvent.IsCloseToTime(nextEvent) {
			nextEvent.Position = MIDDLE
		}
		// Set next item level to current item level
		if currentEvent.Position == START || currentEvent.Position == MIDDLE {
			nextEvent.Level = currentEvent.Level
		}
	} else if currentEvent.Position == MIDDLE {
		currentEvent.Position = END
	}
}
