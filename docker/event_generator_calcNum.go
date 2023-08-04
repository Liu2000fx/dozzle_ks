package docker

import (
	"bufio"
	"bytes"
	"encoding/binary"
 
	"fmt"
 
	"io"
 
	"sync"
 

	log "github.com/sirupsen/logrus"
)

type EventGeneratorCalc struct {
	Errors chan error
	reader *bufio.Reader
	next   *LogEvent
	buffer chan *LogEvent
	tty    bool
	WarnNum int64
	InfoNum int64
	ErrorNum int64
	Num int
	Wg     sync.WaitGroup
}

var bufPoolCalc = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

var BadHeaderErrCalc = fmt.Errorf("dozzle/docker: unable to read header")

func NewEventGeneratorCalc(reader io.Reader, tty bool) *EventGeneratorCalc {
	generator := &EventGeneratorCalc{
		reader: bufio.NewReader(reader),
		buffer: make(chan *LogEvent, 100),
		Errors: make(chan error, 1),
		tty:    tty,
		InfoNum: 0,
		WarnNum: 0,
		ErrorNum: 0,
		Num:0,
	}
	generator.Wg.Add(1)
	go generator.consumeReader()		// 用于消费数据源的读取器lfx
	return generator
}
 

func (g *EventGeneratorCalc) consumeReader() {
	for {
		message, _, readerError := readEventCalc(g.reader, g.tty)
		if message != "" {
			level := guessLogLevelCalc(message)
			g.Num++
			if level == "info" {
				g.InfoNum ++
			}else if level == "warn" || level == "warning" {
				g.WarnNum ++
			}else if level == "error" {
				g.ErrorNum ++
			}
		}
		if readerError == io.EOF {
			fmt.Println("读到io末尾了")
			if readerError != BadHeaderErrCalc {
				g.Errors <- readerError
			}
			close(g.buffer)
			break
		}else if readerError != nil {
			fmt.Println("读到其他错误了")
			if readerError != BadHeaderErrCalc {
				g.Errors <- readerError
			}
		}
	}
	g.Wg.Done()
}


func readEventCalc(reader *bufio.Reader, tty bool) (string, StdType, error) {
	header := []byte{0, 0, 0, 0, 0, 0, 0, 0}
	buffer := bufPoolCalc.Get().(*bytes.Buffer)
	buffer.Reset()
	defer bufPoolCalc.Put(buffer)
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
			return "", streamType, BadHeaderErrCalc
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

