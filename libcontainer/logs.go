package libcontainer

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

func forwardLogs(p *os.File) {
	defer p.Close()

	type jsonLog struct {
		Level string `json:"level"`
		Msg   string `json:"msg"`
	}

	dec := json.NewDecoder(p)
	for {
		var jl jsonLog

		type logFuncTable map[logrus.Level]func(args ...interface{})
		logMapping := logFuncTable{
			logrus.PanicLevel: logrus.Panic,
			logrus.FatalLevel: logrus.Fatal,
			logrus.ErrorLevel: logrus.Error,
			logrus.WarnLevel:  logrus.Warn,
			logrus.InfoLevel:  logrus.Info,
			logrus.DebugLevel: logrus.Debug,
		}

		err := dec.Decode(&jl)
		if err != nil {
			if err == io.EOF {
				logrus.Debug("child pipe closed")
				return
			}
			logrus.Errorf("json logs decoding error: %+v", err)
			return
		}

		lvl, err := logrus.ParseLevel(jl.Level)
		if err != nil {
			fmt.Printf("parsing error\n")
		}
		if logMapping[lvl] != nil {
			logMapping[lvl](jl.Msg)
		}
	}
}

func rawLogs(p *os.File) {
	defer p.Close()

	data := make([]byte, 128)
	for {
		_, err := p.Read(data)

		if err != nil {
			if err == io.EOF {
				logrus.Debug("child pipe closed")
				return
			}
			return
		}
		fmt.Printf("Read data: %s\n", string(data))
	}
}
