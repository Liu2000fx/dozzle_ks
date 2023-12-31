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
	WarnNum int64
	InfoNum int64
	ErrorNum int64
	Num int64
	TotalNum int64
	timeComp int64
	wg     sync.WaitGroup
}

var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

var BadHeaderErr = fmt.Errorf("dozzle/docker: unable to read header")

func NewEventGenerator(reader io.Reader, tty bool, info, warn, errorn, num, timeC int64) *EventGenerator {
	generator := &EventGenerator{
		reader: bufio.NewReader(reader),
		buffer: make(chan *LogEvent, 100),
		Errors: make(chan error, 1),
		Events: make(chan *LogEvent),
		tty:    tty,
		InfoNum: info,
		WarnNum: warn,
		ErrorNum: errorn,
		Num: num,
		TotalNum: 0,
		timeComp: timeC,
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
				log.WithField("InfoNum", g.InfoNum).WithField("WarnNum", g.WarnNum).WithField("ErrorNum", g.ErrorNum).Debug("第一波结束了")
				close(g.Events)
				break
			}

			current = event
			next = g.peek()
		}

		checkPosition(current, next)


		g.Events <- current
	}
	g.wg.Done()
}
func Comp(condition bool) string {
	if condition {
		return ">"
	} else {
		return "<"
	}
}

func (g *EventGenerator) consumeReader() {
	for {
		
		message, streamType, readerError := readEvent(g.reader, g.tty)
		g.TotalNum++
		if message != "" {
			logEvent := g.createEvent(message, streamType)
			logEvent.Level = guessLogLevel(logEvent)
			// fmt.Println(logEvent.Timestamp,  Comp(logEvent.Timestamp > g.timeComp)   , g.timeComp)
			g.Num++
			if logEvent.Timestamp > g.timeComp && logEvent.Level == "info" {
				g.InfoNum ++
			}else if logEvent.Timestamp > g.timeComp && (logEvent.Level == "warn" || logEvent.Level == "warning") {
				g.WarnNum ++
			}else if logEvent.Timestamp > g.timeComp && logEvent.Level == "error" {
				g.ErrorNum ++
			}
			g.buffer <- logEvent
		}
		if readerError != nil {
			fmt.Println("【读300秒】完了")
			if readerError != BadHeaderErr {
				fmt.Println("【读300时】遇到了【意外错误】", readerError)
				g.Errors <- readerError
			}else if readerError == BadHeaderErr {
				fmt.Println("【读300时】遇到了【意外错误--坏头错误*****】", readerError)
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
		n, err := io.ReadFull(reader, header)
		if err != nil {
			fmt.Println("【读300时】遇到了【意外根源1】", err)
			return "", streamType, err
		}
		if n != 8 {
			message, _ := reader.ReadString('\n')
			fmt.Println("-------------------------", n, header, message)
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
			fmt.Println("【读300时】遇到了【意外根源2】", err)
			return "", streamType, err
		}
		return buffer.String(), streamType, nil
	}
}

func (g *EventGenerator) createEvent(message string, streamType StdType) *LogEvent {
	h := fnv.New32a()
	h.Write([]byte(message))
	logEvent := &LogEvent{Id: h.Sum32(), Message: message, Stream: streamType.String(), InfoNum: g.InfoNum, WarnNum: g.WarnNum, ErrorNum: g.ErrorNum}
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
