package logging

import "github.com/Sirupsen/logrus"

var log = logrus.WithFields(logrus.Fields{
	"service": "gms",
})

func Logger() *logrus.Entry {
	return log
}

func init() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.TextFormatter{ForceColors: true})
}
