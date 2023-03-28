package log15

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

const timeKey = "t"
const lvlKey = "lvl"
const msgKey = "msg"
const callKey = "call"
const errorKey = "LOG15_ERROR"
const reqIDKey = "reqid"

var (
	//global variables
	logLevel     byte //used for UDPLogger, 256 types enough
	logMetaKey   string
	logMetaValue string
)

type Lvl int

const (
	LvlCrit Lvl = iota
	LvlError
	LvlWarn
	LvlInfo
	LvlDebug
)

// Returns the name of a Lvl
func (l Lvl) String() string {
	switch l {
	case LvlDebug:
		return "dbug"
	case LvlInfo:
		return "info"
	case LvlWarn:
		return "warn"
	case LvlError:
		return "eror"
	case LvlCrit:
		return "crit"
	default:
		panic("bad level")
	}
}

// Returns the appropriate Lvl from a string name.
// Useful for parsing command line args and configuration files.
func LvlFromString(lvlString string) (Lvl, error) {
	switch lvlString {
	case "debug", "dbug":
		return LvlDebug, nil
	case "info":
		return LvlInfo, nil
	case "warn":
		return LvlWarn, nil
	case "error", "eror":
		return LvlError, nil
	case "crit":
		return LvlCrit, nil
	default:
		return LvlDebug, fmt.Errorf("Unknown level: %v", lvlString)
	}
}

type Meta int

const (
	Order Meta = iota
	BaseMonitor
)

func (m Meta) String() string {
	switch m {
	case Order:
		return "order"
	case BaseMonitor:
		return "baseMonitor"
	default:
		panic("bad meta")
	}
}

// A Record is what a Logger asks its handler to write
type Record struct {
	Time         time.Time
	Lvl          Lvl
	Msg          string
	MetaK        string
	MetaV        string
	Ctx          []interface{}
	Call         string
	CustomCaller string
	RequestID    string
	KeyNames     RecordKeyNames
}

type RecordKeyNames struct {
	Time  string
	Msg   string
	Lvl   string
	Call  string
	ReqID string
}

// A Logger writes key/value pairs to a Handler
type Logger interface {
	// New returns a new Logger that has this logger's context plus the given context
	New(ctx ...interface{}) Logger

	// GetHandler gets the handler associated with the logger.
	GetHandler() Handler

	// SetHandler updates the logger to write records to the specified handler.
	SetHandler(h Handler)

	// Set setLv value. only level below this can be output --[stevenimi]
	SetOutLevel(l Lvl)

	// Log a message at the given level with context key/value pairs
	Debug(msg string, ctx ...interface{})
	Info(msg string, ctx ...interface{})
	Warn(msg string, ctx ...interface{})
	Error(msg string, ctx ...interface{})
	Crit(msg string, ctx ...interface{})
}

type logger struct {
	ctx []interface{}
	h   *swapHandler
	// keep the set level,only below the level can be output --[stevenmi]
	setLv Lvl
}

func (l *logger) write(msg string, lvl Lvl, ctx []interface{}) {
	if lvl <= l.setLv { //  --[stevenmi]
		// add requestid at log head    -- 2019-9-17
		reqID := ""
		value, ok := GetReqIDForGoroutine()
		if ok {
			reqID = value.(string)
		}

		// caller
		var caller = ""
		_, file, line, ok := runtime.Caller(2)
		if ok {
			file = filepath.Base(file)
			caller = file + ":" + strconv.Itoa(line)
		}

		var ttime time.Time
		loc, err := time.LoadLocation("Asia/Chongqing") //Asia/Chongqing
		if err != nil {
			ttime = time.Now()
		} else {
			ttime = time.Now().In(loc)
		}
		l.h.Log(&Record{
			Time: ttime,
			Lvl:  lvl,
			Msg:  msg,
			Ctx:  newContext(l.ctx, ctx),
			Call: caller,
			KeyNames: RecordKeyNames{
				Time:  timeKey,
				Msg:   msgKey,
				Lvl:   lvlKey,
				Call:  callKey,
				ReqID: reqIDKey,
			},
			RequestID: reqID,
		})
	} // --[stevenmi]
}

func (l *logger) writeMeta(msg string, lvl Lvl, metaType Meta, metaData interface{}, ctx []interface{}) {
	if lvl <= l.setLv {
		metaK := metaType.String()
		metaV := formatLogfmtValue(metaData)

		newCtx := make([]interface{}, 0, len(ctx)+2)
		newCtx = append(newCtx, metaK)
		newCtx = append(newCtx, metaV)
		newCtx = append(newCtx, ctx...)

		// add requestid at log head    -- 2019-9-17
		reqID := ""
		value, ok := GetReqIDForGoroutine()
		if ok {
			reqID = value.(string)
		}

		// caller
		var caller = ""
		_, file, line, ok := runtime.Caller(2)
		if ok {
			file = filepath.Base(file)
			caller = file + ":" + strconv.Itoa(line)
		}

		l.h.Log(&Record{
			Time:  time.Now(),
			Lvl:   lvl,
			Msg:   msg,
			MetaK: metaK,
			MetaV: metaV,
			Ctx:   newContext(l.ctx, newCtx),
			Call:  caller,
			KeyNames: RecordKeyNames{
				Time:  timeKey,
				Msg:   msgKey,
				Lvl:   lvlKey,
				Call:  callKey,
				ReqID: reqIDKey,
			},
			RequestID: reqID,
		})
	}
}

func (l *logger) writeGorm(msg string, lvl Lvl, caller string, ctx []interface{}) {
	if lvl <= l.setLv {
		newCtx := make([]interface{}, 0, len(ctx))
		newCtx = append(newCtx, ctx...)

		l.h.Log(&Record{
			Time:         time.Now(),
			Lvl:          lvl,
			Msg:          msg,
			Ctx:          newContext(l.ctx, newCtx),
			CustomCaller: caller,
			KeyNames: RecordKeyNames{
				Time:  timeKey,
				Msg:   msgKey,
				Lvl:   lvlKey,
				Call:  callKey,
				ReqID: reqIDKey,
			},
		})
	}
}

func (l *logger) New(ctx ...interface{}) Logger {
	//child := &logger{newContext(l.ctx, ctx), new(swapHandler)}  // increase one parament --[stevenmi]
	child := &logger{newContext(l.ctx, ctx), new(swapHandler), LvlDebug}
	child.SetHandler(l.h)
	return child
}

func newContext(prefix []interface{}, suffix []interface{}) []interface{} {
	normalizedSuffix := normalize(suffix)
	newCtx := make([]interface{}, len(prefix)+len(normalizedSuffix))
	n := copy(newCtx, prefix)
	copy(newCtx[n:], normalizedSuffix)
	return newCtx
}

// implement , set the Level --[stevenmi]
func (l *logger) SetOutLevel(level Lvl) {
	if level >= LvlCrit && level <= LvlDebug {
		l.setLv = level
	}
	return
}

func (l *logger) Debug(msg string, ctx ...interface{}) {
	l.write(msg, LvlDebug, ctx)
}

func (l *logger) Info(msg string, ctx ...interface{}) {
	l.write(msg, LvlInfo, ctx)
}

func (l *logger) Warn(msg string, ctx ...interface{}) {
	l.write(msg, LvlWarn, ctx)
}

func (l *logger) Error(msg string, ctx ...interface{}) {
	l.write(msg, LvlError, ctx)
}

func (l *logger) Crit(msg string, ctx ...interface{}) {
	l.write(msg, LvlCrit, ctx)
}

func (l *logger) GetHandler() Handler {
	return l.h.Get()
}

func (l *logger) SetHandler(h Handler) {
	l.h.Swap(h)
}

func normalize(ctx []interface{}) []interface{} {
	// if the caller passed a Ctx object, then expand it
	if len(ctx) == 1 {
		if ctxMap, ok := ctx[0].(Ctx); ok {
			ctx = ctxMap.toArray()
		}
	}

	// ctx needs to be even because it's a series of key/value pairs
	// no one wants to check for errors on logging functions,
	// so instead of erroring on bad input, we'll just make sure
	// that things are the right length and users can fix bugs
	// when they see the output looks wrong
	if len(ctx)%2 != 0 {
		ctx = append(ctx, nil, errorKey, "Normalized odd number of arguments by adding nil")
	}

	return ctx
}

// Lazy allows you to defer calculation of a logged value that is expensive
// to compute until it is certain that it must be evaluated with the given filters.
//
// Lazy may also be used in conjunction with a Logger's New() function
// to generate a child logger which always reports the current value of changing
// state.
//
// You may wrap any function which takes no arguments to Lazy. It may return any
// number of values of any type.
type Lazy struct {
	Fn interface{}
}

// Ctx is a map of key/value pairs to pass as context to a log function
// Use this only if you really need greater safety around the arguments you pass
// to the logging functions.
type Ctx map[string]interface{}

func (c Ctx) toArray() []interface{} {
	arr := make([]interface{}, len(c)*2)

	i := 0
	for k, v := range c {
		arr[i] = k
		arr[i+1] = v
		i += 2
	}

	return arr
}
