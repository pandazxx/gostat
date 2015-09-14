package gostat

import (
	"fmt"
	"time"
)

type cmdType int

const (
	cmdDurationBegin cmdType = iota
	cmdDurationEnd
	cmdAddCount
	cmdGetCount
	cmdGetDuration
	cmdAddDuration
	cmdEndSession
)

type statCommand struct {
	CmdType  cmdType
	Key      string
	Value    interface{}
	ResultCh chan interface{}
}

type GoStat struct {
	durationStat map[string]time.Duration
	tmpDuration  map[string]time.Time
	countStat    map[string]int64
	cmdCh        chan statCommand
}

type statSession struct {
	stat             *GoStat
	tmpDurationBegin map[string]time.Time
	duration         map[string]time.Duration
	count            map[string]int64
}

func addTimeToMap(m map[string]time.Time, k string, t time.Time) {
	_, found := m[k]
	if found {
		panic(fmt.Sprintf("Key %v is already in the map", k))
	}
	m[k] = t
}

func getDurationFromMap(m map[string]time.Time, k string, t time.Time) time.Duration {
	t1, found := m[k]
	if !found {
		panic(fmt.Sprintf("Key %v is not in the map", k))
	}
	delete(m, k)
	return t.Sub(t1)
}

func addDurationToMap(m map[string]time.Duration, k string, d time.Duration) {
	sumDuration, found := m[k]
	if !found {
		sumDuration = 0
	}

	sumDuration += d
	m[k] = sumDuration
}

func addCountToMap(m map[string]int64, k string, count int64) {
	sum, _ := m[k]
	sum += count
	m[k] = sum
}

func (s *statSession) DurationBegin(key string) {
	addTimeToMap(s.tmpDurationBegin, key, time.Now())
}

func (s *statSession) DurationEnd(key string) {
	d := getDurationFromMap(s.tmpDurationBegin, key, time.Now())
	addDurationToMap(s.duration, key, d)
}

func (s *statSession) AddCount(key string, count int64) {
	addCountToMap(s.count, key, count)
}

func (s *statSession) finishDurations() {
	for k, v := range s.tmpDurationBegin {
		addDurationToMap(s.duration, k, time.Since(v))
	}
}

func (s *statSession) End() {
	fmt.Printf("stat session end\n")
	s.finishDurations()
	for k, v := range s.duration {
		s.stat.AddDruation(k, v)
	}

	for k, v := range s.count {
		s.stat.AddCount(k, v)
	}
}

func (g *GoStat) start() {
	go func() {
		for cmd := range g.cmdCh {
			// if cmd.CmdType == cmdDurationBegin {
			// 	g.cmdDurationBegin(cmd)
			// } else if cmd.CmdType == cmdDurationEnd {
			// 	g.cmdDurationEnd(cmd)
			// } else if cmd.CmdType == cmdAddCount {
			// 	g.cmdAddCount(cmd)
			// } else if cmd.CmdType == cmdGetCount {
			// 	g.cmdGetCount(cmd)
			// } else if cmd.CmdType == cmdGetDuration {
			// 	g.cmdGetDuration(cmd)
			// }
			switch cmd.CmdType {
			case cmdDurationBegin:
				g.cmdDurationBegin(cmd)
			case cmdDurationEnd:
				g.cmdDurationEnd(cmd)
			case cmdAddCount:
				g.cmdAddCount(cmd)
			case cmdGetCount:
				g.cmdGetCount(cmd)
			case cmdGetDuration:
				g.cmdGetDuration(cmd)
			case cmdAddDuration:
				g.cmdAddDuration(cmd)
			}
		}
	}()
}

func (g *GoStat) NewSession() *statSession {
	return &statSession{
		stat:             g,
		duration:         make(map[string]time.Duration),
		count:            make(map[string]int64),
		tmpDurationBegin: make(map[string]time.Time),
	}
}

func (g *GoStat) cmdAddDuration(cmd statCommand) {
	val, found := g.durationStat[cmd.Key]
	if !found {
		val = time.Duration(0)
	}

	val += cmd.Value.(time.Duration)
	g.durationStat[cmd.Key] = val
}

func (g *GoStat) cmdGetDuration(cmd statCommand) {
	val, found := g.durationStat[cmd.Key]
	if !found {
		val = time.Duration(0)
	}

	cmd.ResultCh <- val
}

func (g *GoStat) cmdGetCount(cmd statCommand) {
	val, found := g.countStat[cmd.Key]
	if !found {
		val = 0
	}

	cmd.ResultCh <- val
}

func (g *GoStat) Stop() {
	close(g.cmdCh)
}

func (g *GoStat) cmdDurationBegin(cmd statCommand) {
	if _, found := g.tmpDuration[cmd.Key]; found {
		panic(fmt.Sprintf("Duplicate duration start for key %v", cmd.Key))
	}

	g.tmpDuration[cmd.Key] = cmd.Value.(time.Time)
}

func (g *GoStat) cmdDurationEnd(cmd statCommand) {
	durationBegin, found := g.tmpDuration[cmd.Key]
	if !found {
		panic(fmt.Sprintf("Daling duration end for key %v", cmd.Key))
	}
	duration := time.Since(durationBegin)

	durationSum, found := g.durationStat[cmd.Key]
	if !found {
		durationSum = 0.0
	}

	g.durationStat[cmd.Key] = durationSum + duration
}

func (g *GoStat) cmdAddCount(cmd statCommand) {
	sumCnt, found := g.countStat[cmd.Key]
	if !found {
		sumCnt = 0
	}
	sumCnt += cmd.Value.(int64)
	g.countStat[cmd.Key] = sumCnt
}

func NewStat() *GoStat {
	ret := &GoStat{
		durationStat: make(map[string]time.Duration),
		countStat:    make(map[string]int64),
		tmpDuration:  make(map[string]time.Time),
		cmdCh:        make(chan statCommand, 20),
	}
	ret.start()
	return ret
}

func (g *GoStat) DurationBegin(key string) {
	g.cmdCh <- statCommand{
		CmdType: cmdDurationBegin,
		Key:     key,
		Value:   time.Now(),
	}
}

func (g *GoStat) DurationEnd(key string) {
	g.cmdCh <- statCommand{
		CmdType: cmdDurationEnd,
		Key:     key,
	}
}

func (g *GoStat) AddCount(key string, value int64) {
	g.cmdCh <- statCommand{
		CmdType: cmdAddCount,
		Key:     key,
		Value:   value,
	}
}

func (g *GoStat) GetCount(key string) int64 {
	resultCh := make(chan interface{})
	defer close(resultCh)
	g.cmdCh <- statCommand{
		CmdType:  cmdGetCount,
		Key:      key,
		ResultCh: resultCh,
	}

	return (<-resultCh).(int64)
}

func (g *GoStat) GetDuration(key string) time.Duration {
	resultCh := make(chan interface{})
	defer close(resultCh)
	g.cmdCh <- statCommand{
		CmdType:  cmdGetDuration,
		Key:      key,
		ResultCh: resultCh,
	}

	return (<-resultCh).(time.Duration)
}

func (g *GoStat) AddDruation(key string, value time.Duration) {
	g.cmdCh <- statCommand{
		CmdType: cmdAddDuration,
		Key:     key,
		Value:   value,
	}
}
