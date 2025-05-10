package btlog

import (
	btzap "github.com/bt-smart/btlog/zap"
)

var Logger *btzap.Logger

func SetLogger(l *btzap.Logger) {
	Logger = l
}
