package docker
import (
	"bufio"
	"fmt"
	"io"
	"sync"
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
	Num int64
	Wg     sync.WaitGroup
}

var BadHeaderErrCalc = fmt.Errorf("dozzle/docker: unable to read header")

func NewEventGeneratorCalc(reader io.Reader, tty bool) *EventGeneratorCalc {
	generator := &EventGeneratorCalc{
		reader: bufio.NewReader(reader),
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
			fmt.Println("【统计计数】:读到io末尾了")
			break
		}else if readerError != nil {
			fmt.Println("【统计计数】:读到其他错误了")
			if readerError != BadHeaderErrCalc {
				g.Errors <- readerError
			}
		}
	}
	g.Wg.Done()
}


func readEventCalc(reader *bufio.Reader, tty bool) (string, StdType, error) {
	message, err := reader.ReadString('\n')
	if err != nil {
		return message, STDOUT, err
	}
	return message, STDOUT, nil
}