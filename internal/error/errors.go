package errorx

import "errors"

var ErrNotEnoughSpace = errors.New("not enough storage space")
var ErrNotAPlusMember = errors.New("not a plus member: access to plus features denied")
var ErrDeviceLimitExceeded = errors.New("device limit exceeded: user can register maximum 5 devices")
