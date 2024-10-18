package main

import (
	"fmt"
	"math"
	"os"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Report struct {
	clients int
	mu      sync.RWMutex

	requests int
	latency  []int
	errors   int

	errExample error
}

type EventSender struct {
	logger         *zap.Logger
	report         *Report
	eventInterval  time.Duration
	timeToFullLoad time.Duration
	reportInterval time.Duration
	maxSenders     int
}

// event loader with linear scaaling
func NewEventSender(eventFuncList []func() error, eventInterval, timeToFullLoad, reportInterval time.Duration) *EventSender {
	// Custom time encoder function for tab alignment
	timeEncoder := func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("02-Jan 15:04:05"))
	}

	// Custom duration encoder function to round to two decimal places
	durationEncoder := func(d time.Duration) string {
		if d >= time.Second {
			return d.String()
		}
		return fmt.Sprintf("%.2fms", float64(d.Microseconds())/1000)
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:       "Time",
		LevelKey:      "Level",
		MessageKey:    "Message",
		StacktraceKey: "",
		EncodeTime:    timeEncoder,
		EncodeLevel:   zapcore.CapitalColorLevelEncoder,
		EncodeDuration: func(d time.Duration, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(durationEncoder(d))
		},
		EncodeCaller: zapcore.ShortCallerEncoder,
	}

	// Create a new logger instance
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.AddSync(os.Stdout),
		zapcore.DebugLevel,
	)

	logger := zap.New(core)

	defer logger.Sync()

	report := &Report{
		latency: make([]int, 0),
		mu:      sync.RWMutex{},
	}

	return &EventSender{
		logger:         logger,
		report:         report,
		eventInterval:  eventInterval,
		timeToFullLoad: timeToFullLoad,
		reportInterval: reportInterval,
	}
}

func (s *EventSender) Start(eventFuncList []func() error) {
	s.maxSenders += len(eventFuncList)
	startProcessEveryN := s.timeToFullLoad / time.Duration(s.maxSenders)

	s.logger.Info("Starting senders with linear scaling",
		zap.String("Time to full load", s.timeToFullLoad.String()),
		zap.Int("Max senders", s.maxSenders),
		zap.String("New sender every", startProcessEveryN.String()),
	)

	go func() {
		ticker := time.NewTicker(s.reportInterval)
		for range ticker.C {
			s.logReport()
		}
	}()
	for _, eventFunc := range eventFuncList {
		go func() {
			s.report.clients++
			ticker := time.NewTicker(s.eventInterval)
			s.sendEvent(eventFunc)
			for range ticker.C {
				s.sendEvent(eventFunc)
			}
		}()
		time.Sleep(startProcessEveryN)
	}
	s.logger.Info("All event senders are alive", zap.Int("senders", s.report.clients))
}

func (s *EventSender) sendEvent(eventFunc func() error) {
	start := time.Now()
	s.report.requests++
	err := eventFunc()
	elapsed := time.Since(start)
	if err != nil {
		s.report.errors++
		s.report.errExample = err
		return
	}

	s.report.mu.Lock()
	s.report.latency = append(s.report.latency, int(elapsed/time.Millisecond))
	s.report.mu.Unlock()
}

func (s *EventSender) logReport() {
	s.report.mu.RLock()
	latency := s.report.latency
	requests := s.report.requests
	errors := s.report.errors
	clients := s.report.clients
	errExample := s.report.errExample
	s.report.mu.RUnlock()

	s.report.mu.Lock()
	s.report.requests = 0
	s.report.latency = make([]int, 0)
	s.report.errExample = nil
	s.report.mu.Unlock()

	avg := calculateAverage(latency)
	med := calculateMedian(latency)
	stddev := calculateStandardDeviation(latency)
	p95 := calculatePercentile(latency, 95)
	p99 := calculatePercentile(latency, 99)

	s.logger.Info("Stats period",
		zap.Duration("period", s.eventInterval),
		zap.Int("clients", clients),
		zap.Int("requests", requests),
		zap.Int("finished req", len(latency)),
		zap.Int("errors req", errors),
		zap.Float64("rps", float64(len(latency))/s.eventInterval.Seconds()),
		zap.Duration("avg", time.Duration(avg)),
		zap.Duration("med", time.Duration(med)),
		zap.Duration("stddev", time.Duration(stddev)),
		zap.Duration("p95", time.Duration(p95)),
		zap.Duration("p99", time.Duration(p99)),
	)
	if errExample != nil {
		s.logger.Error("Error example", zap.Error(errExample))
	}
}

func calculateAverage(numbers []int) float64 {
	if len(numbers) == 0 {
		return 0
	}
	total := 0
	for _, number := range numbers {
		total += number
	}
	return float64(total) / float64(len(numbers))
}

func calculateMedian(numbers []int) float64 {
	if len(numbers) == 0 {
		return 0
	}
	sort.Ints(numbers)
	n := len(numbers)
	if n%2 == 0 {
		return float64(numbers[n/2-1]+numbers[n/2]) / 2.0
	}
	return float64(numbers[n/2])
}

func calculatePercentile(numbers []int, percentile float64) float64 {
	if len(numbers) == 0 {
		return 0
	}
	sort.Ints(numbers)
	index := int(math.Ceil(percentile/100.0*float64(len(numbers)))) - 1
	return float64(numbers[index])
}

func calculateStandardDeviation(numbers []int) float64 {
	if len(numbers) == 0 {
		return 0
	}
	mean := calculateAverage(numbers)
	var sumSquares float64
	for _, number := range numbers {
		sumSquares += (float64(number) - mean) * (float64(number) - mean)
	}
	variance := sumSquares / float64(len(numbers))
	return math.Sqrt(variance)
}
